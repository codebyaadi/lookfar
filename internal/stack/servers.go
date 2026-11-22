package stack

import (
	"sort"
	"strings"
	"unicode"
)

// Server is a curated server-software entry used for CPE-based matching (NVD).
// Vendor and Product follow NVD's CPE dictionary naming.
type Server struct {
	// Name is the canonical name, used in the purl: pkg:generic/<name>@<version>.
	Name    string
	Aliases []string
	Vendor  string
	Product string
}

// servers is the curated list. Entries marked "unsure" were written from
// memory of the NVD CPE dictionary and should be verified against it before
// the NVD collector (M2-3) relies on them.
var servers = []Server{
	{Name: "postgresql", Aliases: []string{"postgres", "pg"}, Vendor: "postgresql", Product: "postgresql"},
	{Name: "mysql", Vendor: "oracle", Product: "mysql"},
	{Name: "mariadb", Vendor: "mariadb", Product: "mariadb"},
	{Name: "redis", Vendor: "redis", Product: "redis"},    // older records use redislabs:redis
	{Name: "valkey", Vendor: "valkey", Product: "valkey"}, // unsure: new project, vendor may be valkey-io
	{Name: "mongodb", Aliases: []string{"mongo"}, Vendor: "mongodb", Product: "mongodb"},
	{Name: "nginx", Vendor: "f5", Product: "nginx"}, // older records use igor_sysoev:nginx
	{Name: "apache-httpd", Aliases: []string{"apache httpd", "httpd", "apache", "apache http server"}, Vendor: "apache", Product: "http_server"},
	{Name: "caddy", Vendor: "caddyserver", Product: "caddy"}, // unsure
	{Name: "haproxy", Vendor: "haproxy", Product: "haproxy"},
	{Name: "traefik", Vendor: "traefik", Product: "traefik"}, // unsure: may be traefik_labs
	{Name: "nodejs", Aliases: []string{"node.js", "node"}, Vendor: "nodejs", Product: "node.js"},
	{Name: "python", Aliases: []string{"cpython"}, Vendor: "python", Product: "python"},
	{Name: "openssl", Vendor: "openssl", Product: "openssl"},
	{Name: "openssh", Vendor: "openbsd", Product: "openssh"},
	{Name: "rabbitmq", Vendor: "vmware", Product: "rabbitmq"}, // unsure: pivotal_software, vmware or broadcom depending on year
	{Name: "elasticsearch", Aliases: []string{"elastic search"}, Vendor: "elastic", Product: "elasticsearch"},
	{Name: "kafka", Aliases: []string{"apache kafka"}, Vendor: "apache", Product: "kafka"},
	{Name: "memcached", Vendor: "memcached", Product: "memcached"},
	{Name: "grafana", Vendor: "grafana", Product: "grafana"},
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
	return "cpe:2.3:a:" + s.Vendor + ":" + s.Product + ":" + escapeCPE(version) + ":*:*:*:*:*:*:*"
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
