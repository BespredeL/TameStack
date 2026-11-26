package context

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

	"tamestack/internal/db"
)

var ipv4Pattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

var commonOSUsers = map[string]bool{
	"root": true, "ubuntu": true, "debian": true, "admin": true,
	"www-data": true, "nginx": true, "apache": true, "centos": true,
	"ec2-user": true, "rocky": true, "alma": true,
}

// IsLocalEndpoint reports whether baseURL points to loopback or a private LAN host.
func IsLocalEndpoint(baseURL string) bool {
	raw := strings.TrimSpace(baseURL)
	if raw == "" {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || host == "0.0.0.0" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
	}
	return false
}

// IsCloudProvider reports whether providerType/baseURL sends requests to an external cloud API.
func IsCloudProvider(providerType, baseURL string) bool {
	p := strings.ToLower(strings.TrimSpace(providerType))
	if p == "" || p == "ollama" || p == "llamacpp" {
		return false
	}
	if IsLocalEndpoint(baseURL) {
		return false
	}
	return true
}

// AliasMasker replaces sensitive infrastructure identifiers (server names, hosts/IPs,
// domains, custom usernames) with deterministic aliases before sending prompts to cloud
// LLMs, and restores the original values in responses and tool arguments.
type AliasMasker struct {
	mu           sync.Mutex
	active       bool
	origToAlias  map[string]string
	aliasToOrig  map[string]string
	nextDynamic  int
}

// NewAliasMasker constructs an AliasMasker populated from servers and sites in database.
func NewAliasMasker(ctx context.Context, database *db.DB, providerType, baseURL string, enabled bool) *AliasMasker {
	m := &AliasMasker{
		active:      enabled && IsCloudProvider(providerType, baseURL),
		origToAlias: make(map[string]string),
		aliasToOrig: make(map[string]string),
		nextDynamic: 50,
	}
	if !m.active || database == nil {
		return m
	}

	servers, err := database.ListServers(ctx)
	if err != nil {
		return m
	}
	sort.Slice(servers, func(i, j int) bool { return servers[i].ID < servers[j].ID })

	srvIdx, hostIdx, userIdx, siteIdx, aliasIdx := 1, 1, 1, 1, 1
	for _, srv := range servers {
		if srv == nil {
			continue
		}
		if name := strings.TrimSpace(srv.Name); len(name) >= 2 {
			m.registerPair(name, fmt.Sprintf("server-%d", srvIdx))
			srvIdx++
		}
		if host := strings.TrimSpace(srv.Host); host != "" && !isExemptIP(host) {
			if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
				m.registerPair(host, fmt.Sprintf("198.51.100.%d", hostIdx))
			} else {
				m.registerPair(host, fmt.Sprintf("host-%d.internal", hostIdx))
			}
			hostIdx++
		}
		if uname := strings.TrimSpace(srv.Username); uname != "" && !commonOSUsers[strings.ToLower(uname)] && len(uname) >= 3 {
			m.registerPair(uname, fmt.Sprintf("ops-user-%d", userIdx))
			userIdx++
		}

		sites, sErr := database.ListSites(ctx, srv.ID)
		if sErr == nil {
			sort.Slice(sites, func(i, j int) bool { return sites[i].ID < sites[j].ID })
			for _, st := range sites {
				if st == nil {
					continue
				}
				if dom := strings.TrimSpace(st.Domain); dom != "" {
					m.registerPair(dom, fmt.Sprintf("site-%d.example.test", siteIdx))
					siteIdx++
				}
				for _, al := range st.Aliases {
					al = strings.TrimSpace(al)
					if al != "" {
						m.registerPair(al, fmt.Sprintf("alias-%d.example.test", aliasIdx))
						aliasIdx++
					}
				}
			}
		}
	}

	return m
}

func isExemptIP(ipStr string) bool {
	if ipStr == "127.0.0.1" || ipStr == "0.0.0.0" || ipStr == "255.255.255.255" {
		return true
	}
	if strings.HasPrefix(ipStr, "198.51.100.") {
		return true
	}
	return false
}

func (m *AliasMasker) registerPair(orig, alias string) {
	if orig == "" || alias == "" {
		return
	}
	if _, exists := m.origToAlias[orig]; exists {
		return
	}
	m.origToAlias[orig] = alias
	m.aliasToOrig[alias] = orig
}

// Active reports whether masking is active for this conversation.
func (m *AliasMasker) Active() bool {
	return m != nil && m.active
}

// Mask replaces known sensitive values and any non-loopback IPv4 addresses with aliases.
func (m *AliasMasker) Mask(input string) string {
	if !m.Active() || input == "" {
		return input
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Dynamically alias any newly discovered IPv4 addresses (e.g. from tool logs/outputs).
	for _, match := range ipv4Pattern.FindAllString(input, -1) {
		if isExemptIP(match) {
			continue
		}
		if ip := net.ParseIP(match); ip != nil && ip.To4() != nil {
			if _, exists := m.origToAlias[match]; !exists {
				alias := fmt.Sprintf("198.51.100.%d", m.nextDynamic)
				m.nextDynamic++
				m.origToAlias[match] = alias
				m.aliasToOrig[alias] = match
			}
		}
	}

	return replaceLongestFirst(input, m.origToAlias)
}

// Unmask restores all aliases in input back to their original values.
func (m *AliasMasker) Unmask(input string) string {
	if !m.Active() || input == "" {
		return input
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return replaceLongestFirst(input, m.aliasToOrig)
}

func replaceLongestFirst(input string, mapping map[string]string) string {
	if len(mapping) == 0 {
		return input
	}
	keys := make([]string, 0, len(mapping))
	for k := range mapping {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})

	pairs := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		pairs = append(pairs, k, mapping[k])
	}
	return strings.NewReplacer(pairs...).Replace(input)
}

// StreamUnmasker buffers partial alias prefixes across streaming deltas so aliases
// split across token boundaries are cleanly restored before reaching the UI.
type StreamUnmasker struct {
	masker  *AliasMasker
	pending string
}

// NewStreamUnmasker creates a streaming unmasker bound to m.
func (m *AliasMasker) NewStreamUnmasker() *StreamUnmasker {
	return &StreamUnmasker{masker: m}
}

// Feed processes a streaming delta and returns the safe unmasked prefix.
func (su *StreamUnmasker) Feed(delta string) string {
	if su == nil || !su.masker.Active() {
		return delta
	}
	if delta == "" {
		return ""
	}
	su.pending += delta

	su.masker.mu.Lock()
	aliases := make([]string, 0, len(su.masker.aliasToOrig))
	for a := range su.masker.aliasToOrig {
		aliases = append(aliases, a)
	}
	su.masker.mu.Unlock()

	if len(aliases) == 0 {
		out := su.pending
		su.pending = ""
		return out
	}

	// Find earliest index in pending that could be the start of a partial alias.
	holdFrom := len(su.pending)
	for i := max(0, len(su.pending)-48); i < len(su.pending); i++ {
		tail := su.pending[i:]
		for _, a := range aliases {
			if strings.HasPrefix(a, tail) {
				if i < holdFrom {
					holdFrom = i
				}
				break
			}
		}
	}

	if holdFrom == 0 {
		return ""
	}
	emit := su.pending[:holdFrom]
	su.pending = su.pending[holdFrom:]
	return su.masker.Unmask(emit)
}

// Flush unmasks and returns any remaining buffered text at the end of a stream.
func (su *StreamUnmasker) Flush() string {
	if su == nil || su.pending == "" {
		return ""
	}
	out := su.masker.Unmask(su.pending)
	su.pending = ""
	return out
}
