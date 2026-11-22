package stack

import (
	"sort"
	"strings"
	"unicode"
)

// CPEPair is a vendor and product as spelled in NVD's CPE dictionary.
type CPEPair struct{ Vendor, Product string }

// Server is a curated server-software entry used for CPE-based matching (NVD).
type Server struct {
	// Name is the canonical name, used in the purl: pkg:generic/<name>@<version>.
	Name    string
	Aliases []string
	// CPEs lists every vendor:product spelling NVD uses for this software,
	// current name first (it becomes Component.CPE). Older CVEs often use a
	// historic vendor name, so all of them are queried and matched.
	CPEs []CPEPair
}

// Primary returns the current vendor:product pair.
func (s Server) Primary() CPEPair { return s.CPEs[0] }

// servers is the curated list.
//
// The CPE pairs were checked against the live NVD CPE-match API on 2026-11-22
// (virtualMatchString=cpe:2.3:a:<vendor>:<product>, totalResults in brackets):
//
//	valkey:valkey [0]  valkey-io:valkey [0]   no CPE exists yet; both are kept so
//	                                          matching starts working once NVD adds one
//	caddyserver:caddy [20]  caddy:caddy [0]
//	traefik:traefik [84]    traefik_labs:traefik [0]
//	vmware:rabbitmq [6]  pivotal_software:rabbitmq [11]  broadcom:rabbitmq [0]  rabbitmq:rabbitmq [0]
//	redis:redis [46]  redislabs:redis [22]
//	f5:nginx [41]  nginx:nginx [2]  igor_sysoev:nginx [0]
//
// and, for the rest: postgresql:postgresql [209], oracle:mysql [1328],
// mariadb:mariadb [431], mongodb:mongodb [189], apache:http_server [372],
// haproxy:haproxy [36], nodejs:node.js [189], python:python [145],
// openssl:openssl [330], openbsd:openssh [147], elastic:elasticsearch [91],
// apache:kafka [17], memcached:memcached [21], grafana:grafana [104].
var servers = []Server{
	{Name: "postgresql", Aliases: []string{"postgres", "pg"}, CPEs: cpes("postgresql:postgresql")},
	{Name: "mysql", CPEs: cpes("oracle:mysql")},
	{Name: "mariadb", CPEs: cpes("mariadb:mariadb")},
	{Name: "redis", CPEs: cpes("redis:redis", "redislabs:redis")},
	{Name: "valkey", CPEs: cpes("valkey:valkey", "valkey-io:valkey")},
	{Name: "mongodb", Aliases: []string{"mongo"}, CPEs: cpes("mongodb:mongodb")},
	{Name: "nginx", CPEs: cpes("f5:nginx", "nginx:nginx")},
	{Name: "apache-httpd", Aliases: []string{"apache httpd", "httpd", "apache", "apache http server"}, CPEs: cpes("apache:http_server")},
	{Name: "caddy", CPEs: cpes("caddyserver:caddy")},
	{Name: "haproxy", CPEs: cpes("haproxy:haproxy")},
	{Name: "traefik", CPEs: cpes("traefik:traefik")},
	{Name: "nodejs", Aliases: []string{"node.js", "node"}, CPEs: cpes("nodejs:node.js")},
	{Name: "python", Aliases: []string{"cpython"}, CPEs: cpes("python:python")},
	{Name: "openssl", CPEs: cpes("openssl:openssl")},
	{Name: "openssh", CPEs: cpes("openbsd:openssh")},
	{Name: "rabbitmq", CPEs: cpes("vmware:rabbitmq", "pivotal_software:rabbitmq")},
	{Name: "elasticsearch", Aliases: []string{"elastic search"}, CPEs: cpes("elastic:elasticsearch")},
	{Name: "kafka", Aliases: []string{"apache kafka"}, CPEs: cpes("apache:kafka")},
	{Name: "memcached", CPEs: cpes("memcached:memcached")},
	{Name: "grafana", CPEs: cpes("grafana:grafana")},
}

// cpes builds pairs from "vendor:product" strings.
func cpes(specs ...string) []CPEPair {
	out := make([]CPEPair, len(specs))
	for i, s := range specs {
		v, p, _ := strings.Cut(s, ":")
		out[i] = CPEPair{Vendor: v, Product: p}
	}
	return out
}

var serverIndex = buildServerIndex()

func buildServerIndex() map[string]*Server {
	idx := map[string]*Server{}
	for i := range servers {
		s := &servers[i]
		idx[normalizeName(s.Name)] = s
		for _, a := range s.Aliases {
			idx[normalizeName(a)] = s
		}
	}
	return idx
}

// normalizeName lowercases and drops everything except letters and digits, so
// "Node.js", "node-js" and "nodejs" all compare equal.
func normalizeName(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// LookupServer finds a curated entry by canonical name or alias.
func LookupServer(name string) (Server, bool) {
	s, ok := serverIndex[normalizeName(name)]
	if !ok {
		return Server{}, false
	}
	return *s, true
}

// ServerNames returns the canonical names of the curated list, sorted.
func ServerNames() []string {
	names := make([]string, len(servers))
	for i, s := range servers {
		names[i] = s.Name
	}
	sort.Strings(names)
	return names
}

// CPE returns the CPE 2.3 formatted string for this server at a version
// ("*" when the version is unknown).
func (s Server) CPE(version string) string {
	if version == "" {
		version = "*"
	}
	p := s.Primary()
	return "cpe:2.3:a:" + p.Vendor + ":" + p.Product + ":" + escapeCPE(version) + ":*:*:*:*:*:*:*"
}

// escapeCPE escapes characters that are special in CPE 2.3 formatted strings.
func escapeCPE(v string) string {
	var b strings.Builder
	for _, r := range v {
		if strings.ContainsRune(`\:/!"#$%&'()+,;<=>@[]^`+"`"+`{|}~?`, r) {
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
