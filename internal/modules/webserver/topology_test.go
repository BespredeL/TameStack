package webserver

import (
	"testing"
)

func TestParseListeningSockets(t *testing.T) {
	sampleSS := `
Netid State  Recv-Q Send-Q Local Address:Port Peer Address:PortProcess
tcp   LISTEN 0      511          0.0.0.0:80        0.0.0.0:*    users:(("nginx",pid=1234,fd=6))
tcp   LISTEN 0      511          0.0.0.0:443       0.0.0.0:*    users:(("nginx",pid=1234,fd=7))
tcp   LISTEN 0      128        127.0.0.1:8080      0.0.0.0:*    users:(("apache2",pid=5678,fd=4))
tcp   LISTEN 0      128        127.0.0.1:3306      0.0.0.0:*    users:(("mysqld",pid=999,fd=10))
`
	ports := ParseListeningSockets(sampleSS)
	if len(ports["nginx"]) != 2 {
		t.Fatalf("expected 2 ports for nginx, got %d", len(ports["nginx"]))
	}
	if len(ports["apache2"]) != 1 || ports["apache2"][0] != 8080 {
		t.Fatalf("expected port 8080 for apache2, got %v", ports["apache2"])
	}
}

func TestDetectTopologyFromScan(t *testing.T) {
	// Hybrid scenario: Nginx on 80/443, Apache on 8080.
	portsHybrid := map[string][]int{
		"nginx":   {80, 443},
		"apache2": {8080},
	}
	topo := EvaluateTopology(portsHybrid, true, true)
	if topo.Topology != TopologyHybrid {
		t.Errorf("expected TopologyHybrid, got %s", topo.Topology)
	}
	if topo.FrontendPort != 80 || topo.BackendPort != 8080 {
		t.Errorf("expected front=80, back=8080, got front=%d, back=%d", topo.FrontendPort, topo.BackendPort)
	}

	// Hybrid scenario: Both active, no detected ports
	topoBothActive := EvaluateTopology(map[string][]int{}, true, true)
	if topoBothActive.Topology != TopologyHybrid {
		t.Errorf("expected TopologyHybrid when both active, got %s", topoBothActive.Topology)
	}

	// Nginx only
	portsNginx := map[string][]int{
		"nginx": {80, 443},
	}
	topoNginx := EvaluateTopology(portsNginx, true, false)
	if topoNginx.Topology != TopologyNginxOnly {
		t.Errorf("expected TopologyNginxOnly, got %s", topoNginx.Topology)
	}

	// Apache only
	portsApache := map[string][]int{
		"apache2": {80, 443},
	}
	topoApache := EvaluateTopology(portsApache, false, true)
	if topoApache.Topology != TopologyApacheOnly {
		t.Errorf("expected TopologyApacheOnly, got %s", topoApache.Topology)
	}
}

func TestParsePHPRuntimes(t *testing.T) {
	socketsList := "/run/php/php7.4-fpm.sock\n/run/php/php8.2-fpm.sock\n"
	apacheMods := "dir.load\nnegotiation.load\nphp7.4.load\nsetenvif.load\n"

	info := ParsePHPRuntimesInfo(socketsList, apacheMods)
	if len(info.ActiveFpmSockets) != 2 {
		t.Fatalf("expected 2 fpm sockets, got %d", len(info.ActiveFpmSockets))
	}
	if !info.HasApacheModPHP {
		t.Errorf("expected HasApacheModPHP to be true")
	}
	if info.ModPHPVersion != "7.4" {
		t.Errorf("expected ModPHPVersion '7.4', got '%s'", info.ModPHPVersion)
	}
}

func TestDetectPort80Conflict(t *testing.T) {
	// Apache listening on 80, Nginx selected.
	portsApache := map[string][]int{
		"apache2": {80, 443},
	}
	conflict, warn := DetectPort80Conflict(portsApache, "nginx")
	if !conflict || warn == "" {
		t.Errorf("expected conflict when Apache occupies port 80 and Nginx is selected")
	}

	// Nginx listening on 80, Apache selected.
	portsNginx := map[string][]int{
		"nginx": {80, 443},
	}
	conflict2, warn2 := DetectPort80Conflict(portsNginx, "apache")
	if !conflict2 || warn2 == "" {
		t.Errorf("expected conflict when Nginx occupies port 80 and Apache is selected")
	}

	// No conflict when chosen server already listens on 80.
	conflict3, _ := DetectPort80Conflict(portsNginx, "nginx")
	if conflict3 {
		t.Errorf("expected no conflict when Nginx listens on 80 and Nginx is selected")
	}
}
