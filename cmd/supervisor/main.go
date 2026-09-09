package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

type Supervisor struct {
	docker *client.Client
}

type ContainerInfo struct {
	Name      string            `json:"name"`
	ID        string            `json:"id"`
	Image     string            `json:"image"`
	Status    string            `json:"status"`
	State     string            `json:"state"`
	Ports     []PortMapping     `json:"ports"`
	Created   int64             `json:"created"`
	StartedAt string            `json:"started_at"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type PortMapping struct {
	Private string `json:"private"`
	Public  string `json:"public"`
	Type    string `json:"type"`
}

type SystemInfo struct {
	Containers int             `json:"containers"`
	Running    int             `json:"running"`
	Stopped    int             `json:"stopped"`
	Images     int             `json:"images"`
	DockerVer  string          `json:"docker_version"`
	OS         string          `json:"os"`
	Arch       string          `json:"arch"`
	List       []ContainerInfo `json:"containers_list"`
}

type ServiceInfo struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Active string `json:"active"`
	Sub    string `json:"sub"`
	PID    string `json:"pid"`
}

type NginxSite struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Enabled     bool     `json:"enabled"`
	EnabledPath string   `json:"enabled_path,omitempty"`
	ServerName  []string `json:"server_name,omitempty"`
	Content     string   `json:"content,omitempty"`
}

type NginxDomain struct {
	Name  string   `json:"name"`
	Sites []string `json:"sites"`
}

func extractServerNames(conf string) []string {
	var names []string
	for _, line := range strings.Split(conf, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "server_name") {
			continue
		}
		val := strings.TrimSpace(strings.TrimPrefix(trimmed, "server_name"))
		val = strings.TrimSuffix(val, ";")
		for _, n := range strings.Fields(val) {
			if n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

func (s *Supervisor) handleNginxSites(w http.ResponseWriter, r *http.Request) {
	const dir = "/etc/nginx/sites-available"
	const enabledDir = "/etc/nginx/sites-enabled"

	entries, err := os.ReadDir(dir)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}

	var sites []NginxSite
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		// Only real site configs (contain a `server {` block)
		if !strings.Contains(string(data), "server {") {
			continue
		}

		enabledPath := filepath.Join(enabledDir, e.Name())
		enabled := false
		if _, err := os.Lstat(enabledPath); err == nil {
			enabled = true
		}

		sites = append(sites, NginxSite{
			Name:        e.Name(),
			Path:        path,
			Enabled:     enabled,
			EnabledPath: enabledPath,
			ServerName:  extractServerNames(string(data)),
			Content:     string(data),
		})
	}

	sort.Slice(sites, func(i, j int) bool {
		return sites[i].Name < sites[j].Name
	})

	// Aggregate unique server_name domains across all site configs
	domainMap := map[string]map[string]bool{}
	for _, s := range sites {
		for _, n := range s.ServerName {
			if n == "_" {
				continue
			}
			if _, ok := domainMap[n]; !ok {
				domainMap[n] = map[string]bool{}
			}
			domainMap[n][s.Name] = true
		}
	}
	var domains []NginxDomain
	for name, siteSet := range domainMap {
		var sl []string
		for sn := range siteSet {
			sl = append(sl, sn)
		}
		sort.Strings(sl)
		domains = append(domains, NginxDomain{Name: name, Sites: sl})
	}
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Name < domains[j].Name
	})

	jsonResp(w, 200, map[string]any{
		"sites":             sites,
		"server_names":      domains,
		"server_names_count": len(domains),
	})
}

func NewSupervisor() (*Supervisor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Supervisor{docker: cli}, nil
}

func jsonResp(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	jsonResp(w, status, map[string]string{"error": msg})
}

func (s *Supervisor) listContainers(all bool) ([]ContainerInfo, error) {
	ctx := context.Background()
	containers, err := s.docker.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, err
	}

	var result []ContainerInfo
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		var ports []PortMapping
		for _, p := range c.Ports {
			pm := PortMapping{
				Private: fmt.Sprintf("%d/%s", p.PrivatePort, p.Type),
				Type:    p.Type,
			}
			if p.PublicPort > 0 {
				pm.Public = fmt.Sprintf("%s:%d/%s", p.IP, p.PublicPort, p.Type)
			}
			ports = append(ports, pm)
		}

		info := ContainerInfo{
			Name:    name,
			ID:      c.ID[:12],
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Ports:   ports,
			Created: c.Created,
			Labels:  c.Labels,
		}
		result = append(result, info)
	}
	return result, nil
}

func (s *Supervisor) handleListContainers(w http.ResponseWriter, r *http.Request) {
	all := r.URL.Query().Get("all") == "true"
	containers, err := s.listContainers(all)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, 200, map[string]any{"containers": containers})
}

func (s *Supervisor) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/container/")
	name = strings.TrimSpace(name)
	if name == "" {
		jsonErr(w, 400, "container name required")
		return
	}

	ctx := context.Background()
	info, err := s.docker.ContainerInspect(ctx, name)
	if err != nil {
		jsonErr(w, 404, err.Error())
		return
	}

	var ports []PortMapping
	for p, bindings := range info.NetworkSettings.Ports {
		pm := PortMapping{
			Private: string(p),
			Type:    string(p.Proto()),
		}
		if len(bindings) > 0 {
			pm.Public = fmt.Sprintf("%s:%s/%s", bindings[0].HostIP, bindings[0].HostPort, p.Proto())
		}
		ports = append(ports, pm)
	}

	jsonResp(w, 200, map[string]any{
		"name":       strings.TrimPrefix(info.Name, "/"),
		"id":         info.ID[:12],
		"image":      info.Config.Image,
		"state":      info.State.Status,
		"running":    info.State.Running,
		"paused":     info.State.Paused,
		"restarts":   info.RestartCount,
		"started_at": info.State.StartedAt,
		"ports":      ports,
		"labels":     info.Config.Labels,
		"env":        info.Config.Env,
		"mounts":     info.Mounts,
	})
}

func (s *Supervisor) handleContainerAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/container/"), "/")
	if len(parts) < 2 {
		jsonErr(w, 400, "usage: /api/container/{name}/{action}")
		return
	}
	name := parts[0]
	action := parts[1]

	ctx := context.Background()
	var err error

	switch action {
	case "start":
		err = s.docker.ContainerStart(ctx, name, container.StartOptions{})
	case "stop":
		timeout := 10
		err = s.docker.ContainerStop(ctx, name, container.StopOptions{Timeout: &timeout})
	case "restart":
		timeout := 10
		err = s.docker.ContainerRestart(ctx, name, container.StopOptions{Timeout: &timeout})
	case "remove":
		err = s.docker.ContainerRemove(ctx, name, container.RemoveOptions{})
	default:
		jsonErr(w, 400, "unknown action: "+action)
		return
	}

	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, 200, map[string]string{"status": "ok", "action": action, "container": name})
}

func (s *Supervisor) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	containers, err := s.listContainers(true)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}

	running := 0
	stopped := 0
	for _, c := range containers {
		if c.State == "running" {
			running++
		} else {
			stopped++
		}
	}

	images, err := s.docker.ImageList(ctx, image.ListOptions{})
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}

	info, err := s.docker.Info(ctx)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}

	jsonResp(w, 200, SystemInfo{
		Containers: len(containers),
		Running:    running,
		Stopped:    stopped,
		Images:     len(images),
		DockerVer:  info.ServerVersion,
		OS:         info.OperatingSystem,
		Arch:       info.Architecture,
		List:       containers,
	})
}

func (s *Supervisor) handlePortMap(w http.ResponseWriter, r *http.Request) {
	containers, err := s.listContainers(true)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}

	type PortEntry struct {
		Container  string `json:"container"`
		Host       string `json:"host"`
		Container_ string `json:"container_port"`
	}

	var ports []PortEntry
	for _, c := range containers {
		for _, p := range c.Ports {
			if p.Public != "" {
				ports = append(ports, PortEntry{
					Container:  c.Name,
					Host:       p.Public,
					Container_: p.Private,
				})
			}
		}
	}

	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Host < ports[j].Host
	})

	jsonResp(w, 200, map[string]any{"port_map": ports})
}

func (s *Supervisor) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/container/"), "/")
	if len(parts) < 2 || parts[1] != "logs" {
		jsonErr(w, 400, "usage: /api/container/{name}/logs?lines=100")
		return
	}
	name := parts[0]

	tail := "100"
	if v := r.URL.Query().Get("lines"); v != "" {
		tail = v
	}

	ctx := context.Background()
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	}

	reader, err := s.docker.ContainerLogs(ctx, name, opts)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "text/plain")
	buf := make([]byte, 4096)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
}

func systemctlRun(action, name string) (string, error) {
	args := []string{action, name}
	if action == "show" {
		args = []string{action, name, "--property=Name,ActiveState,SubState,MainPID"}
	}
	cmd := exec.Command("systemctl", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func systemctlStatus(name string) (*ServiceInfo, error) {
	raw, err := systemctlRun("show", name)
	if err != nil {
		return nil, err
	}
	svc := &ServiceInfo{Name: name}
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "Name":
			svc.Name = parts[1]
		case "ActiveState":
			svc.Active = parts[1]
		case "SubState":
			svc.Sub = parts[1]
		case "MainPID":
			svc.PID = parts[1]
		}
	}
	switch svc.Active {
	case "active":
		svc.State = "running"
	case "inactive", "failed":
		svc.State = "stopped"
	default:
		svc.State = svc.Active
	}
	return svc, nil
}

func discoverServices() []string {
	seen := map[string]bool{}
	var names []string

	// First: all services from /etc/systemd/system/ (custom/installed)
	entries, err := os.ReadDir("/etc/systemd/system")
	if err == nil {
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".service") {
				continue
			}
			// Skip symlinks (they point to /lib system services)
			if e.Type() == os.ModeSymlink {
				continue
			}
			full := filepath.Join("/etc/systemd/system", name)
			info, err := os.Stat(full)
			if err != nil || info.IsDir() {
				continue
			}
			svcName := strings.TrimSuffix(name, ".service")
			if !seen[svcName] {
				seen[svcName] = true
				names = append(names, svcName)
			}
		}
	}

	// Second: known services from /lib/systemd/system/ that we care about
	knownFromLib := []string{"nginx"}
	for _, svc := range knownFromLib {
		if !seen[svc] {
			seen[svc] = true
			names = append(names, svc)
		}
	}

	sort.Strings(names)
	return names
}

func (s *Supervisor) handleListServices(w http.ResponseWriter, r *http.Request) {
	names := discoverServices()
	var services []ServiceInfo
	for _, name := range names {
		svc, err := systemctlStatus(name)
		if err != nil {
			services = append(services, ServiceInfo{Name: name, State: "unknown"})
			continue
		}
		services = append(services, *svc)
	}
	jsonResp(w, 200, map[string]any{"services": services})
}

func (s *Supervisor) handleGetService(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/service/")
	name = strings.TrimSpace(name)
	if name == "" {
		jsonErr(w, 400, "service name required")
		return
	}
	svc, err := systemctlStatus(name)
	if err != nil {
		jsonErr(w, 500, err.Error())
		return
	}
	jsonResp(w, 200, svc)
}

func (s *Supervisor) handleServiceAction(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/service/"), "/")
	if len(parts) < 2 {
		jsonErr(w, 400, "usage: /api/service/{name}/{start|stop|restart}")
		return
	}
	name := parts[0]
	action := parts[1]

	var systemctlAction string
	switch action {
	case "start":
		systemctlAction = "start"
	case "stop":
		systemctlAction = "stop"
	case "restart":
		systemctlAction = "restart"
	default:
		jsonErr(w, 400, "unknown action: "+action)
		return
	}

	out, err := systemctlRun(systemctlAction, name)
	if err != nil {
		jsonErr(w, 500, string(out)+" "+err.Error())
		return
	}

	svc, _ := systemctlStatus(name)
	jsonResp(w, 200, map[string]any{"status": "ok", "action": action, "service": svc})
}

func (s *Supervisor) handleRPC(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Method string   `json:"method"`
		Params []string `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, 400, "invalid json: "+err.Error())
		return
	}
	if req.Method == "" {
		jsonErr(w, 400, "method required")
		return
	}

	args := []string{"-datadir=/home/coin/.legacycoin", req.Method}
	args = append(args, req.Params...)
	cmd := exec.Command("/home/coin/go/bin/legacycoin-cli", args...)
	out, err := cmd.CombinedOutput()

	result := strings.TrimSpace(string(out))
	if err != nil {
		jsonResp(w, 200, map[string]any{"error": result, "ok": false})
		return
	}

	// Try to parse as JSON, return raw string if not valid JSON
	var parsed any
	if json.Unmarshal([]byte(result), &parsed) == nil {
		jsonResp(w, 200, map[string]any{"result": parsed, "ok": true})
	} else {
		jsonResp(w, 200, map[string]any{"result": result, "ok": true})
	}
}

func main() {
	port := getEnv("SUPERVISOR_PORT", "9000")
	host := getEnv("SUPERVISOR_HOST", "0.0.0.0")

	sup, err := NewSupervisor()
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}

	http.HandleFunc("/api/containers", sup.handleListContainers)
	http.HandleFunc("/api/container/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/container/")
		parts := strings.Split(path, "/")

		if len(parts) == 1 || (len(parts) == 2 && parts[1] == "") {
			sup.handleGetContainer(w, r)
		} else if len(parts) == 2 && parts[1] == "logs" {
			sup.handleContainerLogs(w, r)
		} else if len(parts) == 2 {
			sup.handleContainerAction(w, r)
		} else {
			jsonErr(w, 400, "unknown path")
		}
	})
	http.HandleFunc("/api/system", sup.handleSystemInfo)
	http.HandleFunc("/api/ports", sup.handlePortMap)
	http.HandleFunc("/api/services", sup.handleListServices)
	http.HandleFunc("/api/nginx-sites", sup.handleNginxSites)
	http.HandleFunc("/api/service/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/service/")
		parts := strings.Split(path, "/")

		if len(parts) == 1 || (len(parts) == 2 && parts[1] == "") {
			sup.handleGetService(w, r)
		} else if len(parts) == 2 {
			sup.handleServiceAction(w, r)
	} else {
		jsonErr(w, 400, "unknown path")
		}
	})
	http.HandleFunc("/api/rpc", sup.handleRPC)

	addr := host + ":" + port
	fmt.Println("===========================================")
	fmt.Println("  supervisor - Docker + Systemd Manager")
	fmt.Println("===========================================")
	fmt.Printf("  Port:    %s\n", port)
	fmt.Printf("  Host:    %s\n", host)
	fmt.Printf("  Listen:  %s\n", addr)
	fmt.Println("===========================================")
	fmt.Println("  Endpoints:")
	fmt.Println("    GET  /api/system              System overview")
	fmt.Println("    GET  /api/containers          List containers")
	fmt.Println("    GET  /api/container/{name}    Container details")
	fmt.Println("    POST /api/container/{name}/{start|stop|restart|remove}")
	fmt.Println("    GET  /api/container/{name}/logs?lines=100")
	fmt.Println("    GET  /api/ports               Port mapping")
	fmt.Println("    GET  /api/services            List services")
	fmt.Println("    GET  /api/nginx-sites         List nginx sites (sites-available)")
	fmt.Println("    GET  /api/service/{name}      Service status")
	fmt.Println("    POST /api/service/{name}/{start|stop|restart}")
	fmt.Println("    POST /api/rpc                 LegacyCoin RPC proxy")
	fmt.Println("===========================================")

	log.Fatal(http.ListenAndServe(addr, nil))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
