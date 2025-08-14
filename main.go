package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/geoip2-golang"
)

type GeoInfo struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname,omitempty"`
	City     string `json:"city,omitempty"`
	Region   string `json:"region,omitempty"`
	Country  string `json:"country,omitempty"`
	Loc      string `json:"loc,omitempty"`
	Org      string `json:"org,omitempty"`
	Postal   string `json:"postal,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	Bogon    bool   `json:"bogon,omitempty"`
}

type ErrorResponse struct {
	Status int `json:"status"`
	Error  struct {
		Title   string `json:"title"`
		Message string `json:"message"`
	} `json:"error"`
}

type DatabaseManager struct {
	mu     sync.RWMutex
	asnDB  *geoip2.Reader
	cityDB *geoip2.Reader
}

func NewDatabaseManager() *DatabaseManager {
	return &DatabaseManager{}
}

func (dm *DatabaseManager) LoadDatabases() error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.asnDB != nil {
		dm.asnDB.Close()
	}
	if dm.cityDB != nil {
		dm.cityDB.Close()
	}

	if _, err := os.Stat("GeoLite2-ASN.mmdb"); err == nil {
		asnDB, err := geoip2.Open("GeoLite2-ASN.mmdb")
		if err != nil {
			return fmt.Errorf("failed to open ASN database: %v", err)
		}
		dm.asnDB = asnDB
	}

	if _, err := os.Stat("GeoLite2-City.mmdb"); err == nil {
		cityDB, err := geoip2.Open("GeoLite2-City.mmdb")
		if err != nil {
			return fmt.Errorf("failed to open City database: %v", err)
		}
		dm.cityDB = cityDB
	}

	return nil
}

func (dm *DatabaseManager) Close() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.asnDB != nil {
		dm.asnDB.Close()
		dm.asnDB = nil
	}
	if dm.cityDB != nil {
		dm.cityDB.Close()
		dm.cityDB = nil
	}
}

func (dm *DatabaseManager) LookupASN(ip net.IP) (*geoip2.ASN, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.asnDB == nil {
		return nil, fmt.Errorf("ASN database not available")
	}
	return dm.asnDB.ASN(ip)
}

func (dm *DatabaseManager) LookupCity(ip net.IP) (*geoip2.City, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.cityDB == nil {
		return nil, fmt.Errorf("City database not available")
	}
	return dm.cityDB.City(ip)
}

var dbManager = NewDatabaseManager()

var asnOrgMappings = map[uint]string{
	701:    "Verizon Business",
	702:    "Verizon Business",
	703:    "Verizon Business",
	7018:   "AT&T Services",
	1239:   "Sprint",
	3356:   "Level 3 Communications",
	6461:   "Zayo Bandwidth",
	16509:  "Amazon Web Services",
	14618:  "Amazon Web Services",
	8075:   "Microsoft Azure",
	15169:  "Google Cloud",
	13335:  "Cloudflare",
	7922:   "Comcast Cable",
	11427:  "Charter Communications",
	20001:  "Charter Communications",
	33650:  "Comcast Cable",
	33651:  "Comcast Cable",
	33652:  "Comcast Cable",
	33653:  "Comcast Cable",
	33654:  "Comcast Cable",
	33655:  "Comcast Cable",
	33656:  "Comcast Cable",
	33657:  "Comcast Cable",
	33658:  "Comcast Cable",
	33659:  "Comcast Cable",
	33660:  "Comcast Cable",
	33661:  "Comcast Cable",
	33662:  "Comcast Cable",
	33663:  "Comcast Cable",
	33664:  "Comcast Cable",
	33665:  "Comcast Cable",
	33666:  "Comcast Cable",
	33667:  "Comcast Cable",
	33668:  "Comcast Cable",
	33669:  "Comcast Cable",
	33670:  "Comcast Cable",
	33671:  "Comcast Cable",
	33672:  "Comcast Cable",
	33673:  "Comcast Cable",
	33674:  "Comcast Cable",
	33675:  "Comcast Cable",
	33676:  "Comcast Cable",
	33677:  "Comcast Cable",
	33678:  "Comcast Cable",
	33679:  "Comcast Cable",
	33680:  "Comcast Cable",
	2906:   "Netflix",
	40027:  "Netflix",
	55095:  "Netflix",
	394406: "Netflix",
	32934:  "Facebook",
	54113:  "Fastly",
	174:    "Cogent Communications",
	1299:   "Telia Company",
	2914:   "NTT Communications",
	3257:   "GTT Communications",
	3320:   "Deutsche Telekom",
	5511:   "Orange",
	6830:   "Liberty Global",
	12956:  "Telefonica",
	209:    "CenturyLink",
	3549:   "Level 3 Communications",
	4323:   "Time Warner Cable",
	5650:   "Frontier Communications",
	6128:   "Cablevision",
	6147:   "Telefonica USA",
	6181:   "Covad Communications",
	6327:   "Shaw Communications",
	6389:   "BellSouth.net",
	6478:   "AT&T Internet Services",
	7843:   "Time Warner Cable",
	10796:  "Charter Communications",
	11351:  "Charter Communications",
	12271:  "Charter Communications",
	20115:  "Charter Communications",
	25983:  "Monkeybrains",
	46887:  "Lightower Fiber Networks",
	19994:  "Rackspace",
	26496:  "GoDaddy",
	29748:  "Serverius",
	36351:  "SoftLayer Technologies",
	63949:  "Linode",
	14061:  "DigitalOcean",
	11537:  "Internet2",
	32:     "Stanford University",
	27:     "Department of Defense",
}

func bogon(ip net.IP) bool {
	if ip.To4() != nil {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
			return true
		}

		bogonRanges := []string{
			"0.0.0.0/8",
			"100.64.0.0/10",
			"127.0.0.0/8",
			"169.254.0.0/16",
			"224.0.0.0/4",
			"240.0.0.0/4",
			"255.255.255.255/32",
		}

		for _, cidr := range bogonRanges {
			_, network, _ := net.ParseCIDR(cidr)
			if network != nil && network.Contains(ip) {
				return true
			}
		}
	} else {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsPrivate() {
			return true
		}

		ipv6BogonRanges := []string{
			"::/128",
			"::1/128",
			"::ffff:0:0/96",
			"64:ff9b::/96",
			"100::/64",
			"2001::/23",
			"2001:db8::/32",
			"fc00::/7",
			"fe80::/10",
			"ff00::/8",
		}

		for _, cidr := range ipv6BogonRanges {
			_, network, _ := net.ParseCIDR(cidr)
			if network != nil && network.Contains(ip) {
				return true
			}
		}
	}

	return false
}

func getIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return strings.TrimSpace(xri)
	}

	cf := r.Header.Get("CF-Connecting-IP")
	if cf != "" {
		return strings.TrimSpace(cf)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

var dbURLs = map[string]string{
	"GeoLite2-ASN.mmdb":     "https://git.io/GeoLite2-ASN.mmdb",
	"GeoLite2-City.mmdb":    "https://git.io/GeoLite2-City.mmdb",
	"GeoLite2-Country.mmdb": "https://git.io/GeoLite2-Country.mmdb",
}

func checkDB() {
	for filename, url := range dbURLs {
		go func(filename, url string) {
			if needsUpdate(filename, url) {
				log.Printf("Updating %s...", filename)
				if err := downloadAndReplaceDatabase(filename, url); err != nil {
					log.Printf("Failed to update %s: %v", filename, err)
				} else {
					log.Printf("Successfully updated %s", filename)
				}
			}
		}(filename, url)
	}
}

func downloadAndReplaceDatabase(filename, url string) error {

	tempFile := filename + ".tmp"
	if err := downloadDatabase(tempFile, url); err != nil {
		return err
	}

	dbManager.Close()

	if err := os.Rename(tempFile, filename); err != nil {
		os.Remove(tempFile)
		dbManager.LoadDatabases()
		return fmt.Errorf("failed to replace file: %v", err)
	}

	if err := dbManager.LoadDatabases(); err != nil {
		log.Printf("Warning: Failed to reload databases after update: %v", err)
	}

	return nil
}

func needsUpdate(filename, url string) bool {
	info, err := os.Stat(filename)
	if err != nil {
		return true
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		log.Printf("Error creating HEAD request for %s: %v", filename, err)
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error checking %s: %v", filename, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error checking %s: HTTP %d", filename, resp.StatusCode)
		return false
	}

	lastModified := resp.Header.Get("Last-Modified")
	if lastModified == "" {
		return time.Since(info.ModTime()) > 7*24*time.Hour
	}

	remoteTime, err := http.ParseTime(lastModified)
	if err != nil {
		log.Printf("Error parsing Last-Modified for %s: %v", filename, err)
		return false
	}

	return remoteTime.After(info.ModTime())
}

func downloadDatabase(filename, url string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		os.Remove(filename)
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

func main() {
	log.Println("Checking for database updates...")
	checkDB()

	time.Sleep(2 * time.Second)

	if err := dbManager.LoadDatabases(); err != nil {
		log.Printf("Warning: Failed to load databases: %v", err)
	}

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			log.Println("Performing periodic database check...")
			checkDB()
		}
	}()

	http.HandleFunc("/", handler)
	fmt.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handler(w http.ResponseWriter, r *http.Request) {
	ipStr := strings.Trim(r.URL.Path, "/")

	if ipStr == "" {
		ipStr = getIP(r)
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)

		errorResp := ErrorResponse{
			Status: 404,
		}
		errorResp.Error.Title = "Wrong ip"
		errorResp.Error.Message = "Please provide a valid IP address"

		jsonData, _ := json.MarshalIndent(errorResp, "", "  ")
		w.Write(jsonData)
		return
	}

	isBogon := bogon(ip)

	info := GeoInfo{
		IP:    ip.String(),
		Bogon: isBogon,
	}

	if !isBogon {
		asn, err := dbManager.LookupASN(ip)
		if err != nil {
			log.Printf("ASN lookup error for %s: %v", ip, err)
		}

		city, err := dbManager.LookupCity(ip)
		if err != nil {
			log.Printf("City lookup error for %s: %v", ip, err)
		}

		if asn != nil {
			orgName := asn.AutonomousSystemOrganization
			if mappedName, exists := asnOrgMappings[asn.AutonomousSystemNumber]; exists {
				orgName = mappedName
			}
			info.Org = fmt.Sprintf("AS%d %s", asn.AutonomousSystemNumber, orgName)
		}

		if city != nil {
			info.City = city.City.Names["en"]
			info.Region = firstSubdivision(city)
			info.Country = city.Country.IsoCode
			info.Loc = fmt.Sprintf("%f,%f", city.Location.Latitude, city.Location.Longitude)
			info.Postal = city.Postal.Code
			info.Timezone = city.Location.TimeZone
		}

		info.Hostname = getHostname(ip)
	}

	w.Header().Set("Content-Type", "application/json")
	jsonData, _ := json.MarshalIndent(info, "", "  ")
	w.Write(jsonData)
}

func getHostname(ip net.IP) string {
	ipStr := ip.String()

	resultChan := make(chan string, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		defer cancel()

		resolver := &net.Resolver{}
		names, err := resolver.LookupAddr(ctx, ipStr)
		if err != nil || len(names) == 0 {
			resultChan <- ""
			return
		}

		hostname := names[0]
		if strings.HasSuffix(hostname, ".") {
			hostname = hostname[:len(hostname)-1]
		}
		resultChan <- hostname
	}()

	select {
	case hostname := <-resultChan:
		return hostname
	case <-time.After(50 * time.Millisecond):
		return ""
	}
}

func firstSubdivision(city *geoip2.City) string {
	if len(city.Subdivisions) > 0 {
		return city.Subdivisions[0].Names["en"]
	}
	return ""
}
