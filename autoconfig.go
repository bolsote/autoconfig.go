package main

import (
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type Server struct {
	Type           string `xml:"type,attr"`
	Hostname       string `xml:"hostname"`
	Port           uint16 `xml:"port"`
	SocketType     string `xml:"socketType"`
	Authentication string `xml:"authentication"`
	Username       string `xml:"username"`
}
type IncomingServer struct {
	Server
}
type OutgoingServer struct {
	Server
}

type Provider struct {
	Id               string `xml:"id,attr"`
	Domain           string `xml:"domain"`
	DisplayName      string `xml:"displayName"`
	DisplayShortName string `xml:"displayShortName"`

	IncomingServers []IncomingServer `xml:"incomingServer"`
	OutgoingServers []OutgoingServer `xml:"outgoingServer"`
}

type ClientConfig struct {
	XMLName xml.Name `xml:"clientConfig"`

	Version   string     `xml:"version,attr"`
	Providers []Provider `xml:"emailProvider"`
}

type Domain struct {
	domain string
	config ClientConfig
}

type DomainConfig interface {
	lookup(service, proto string) (string, uint16, error)
	GenerateXml() ([]byte, error)
	HTTPHandler(w http.ResponseWriter, r *http.Request)
}

// Lookup the given service, protocol pair in the domain SRV records.
func (d *Domain) lookup(service, proto string) (string, uint16, error) {
	_, addresses, err := net.LookupSRV(service, proto, d.domain)

	if err != nil {
		return "", 0, err
	}

	return strings.Trim(addresses[0].Target, "."), addresses[0].Port, nil
}

// Generate an autoconfig XML document based on the information obtained from
// querying the domain SRV records.
func (d *Domain) GenerateXml() ([]byte, error) {
	// Incoming server.
	addressIn, portIn, err := d.lookup("imaps", "tcp")
	if err != nil {
		return nil, err
	}
	incoming := IncomingServer{
		Server{
			Type:           "imap",
			Hostname:       addressIn,
			Port:           portIn,
			SocketType:     "SSL",
			Authentication: "password-cleartext",
			Username:       "%EMAILLOCALPART%",
		},
	}

	// Outgoing server.
	addressOut, portOut, err := d.lookup("submission", "tcp")
	if err != nil {
		return nil, err
	}
	outgoing := OutgoingServer{
		Server{
			Type:           "smtp",
			Hostname:       addressOut,
			Port:           portOut,
			SocketType:     "SSL",
			Authentication: "password-cleartext",
			Username:       "%EMAILLOCALPART%",
		},
	}

	// Final data mangling.
	d.config = ClientConfig{
		Version: "1.1",
		Providers: []Provider{
			Provider{
				Id:               d.domain,
				Domain:           d.domain,
				DisplayName:      d.domain,
				DisplayShortName: d.domain,
				IncomingServers:  []IncomingServer{incoming},
				OutgoingServers:  []OutgoingServer{outgoing},
			},
		},
	}

	xmlconfig, err := xml.Marshal(&d.config)
	if err != nil {
		return nil, err
	}

	return xmlconfig, nil
}

func (d *Domain) HttpHandler(w http.ResponseWriter, r *http.Request) {
	xmlconfig, err := d.GenerateXml()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, xml.Header, string(xmlconfig))
}

func main() {
	domain := &Domain{"marshland.ovh", ClientConfig{}}
	http.HandleFunc("/", domain.HttpHandler)
	http.ListenAndServe(":9090", nil)
}
