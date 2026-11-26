package main

import (
	"tamestack/internal/db"
	"tamestack/internal/modules/dns"
)

func (a *App) GetDNSStatus(serverID string) (*dns.DNSStatus, error) {
	return withLock(a, func() (*dns.DNSStatus, error) {
		return a.dnsService.GetDNSStatus(a.ctx, serverID)
	})
}

func (a *App) InstallBIND9(serverID string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dnsService.InstallBIND9(a.ctx, serverID)
	})
}

func (a *App) CreateDNSZone(serverID, domain, serverIP string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dnsService.CreateZone(a.ctx, serverID, domain, serverIP)
	})
}

func (a *App) GetDNSZone(serverID, domain string) (*dns.DNSZoneDetail, error) {
	return withLock(a, func() (*dns.DNSZoneDetail, error) {
		return a.dnsService.GetZone(a.ctx, serverID, domain)
	})
}

func (a *App) SaveDNSZoneRecords(serverID, domain string, records []dns.DNSRecord) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dnsService.SaveZoneRecords(a.ctx, serverID, domain, records)
	})
}

func (a *App) DeleteDNSZone(serverID, domain string) (*db.TaskRecord, error) {
	return withLock(a, func() (*db.TaskRecord, error) {
		return a.dnsService.DeleteZone(a.ctx, serverID, domain)
	})
}
