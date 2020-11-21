package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"text/template"
)

func readLastIP(fileName string) (string, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return "", err
	}
	defer func(c io.Closer) { _ = f.Close() }(f)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " ")
		if len(line) > 0 {
			return line, nil
		}
	}
	return "", nil
}

func storeCurrentIP(fileName string, current string) error {
	return ioutil.WriteFile(fileName, []byte(current), 0777)
}

var re = regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)

func doGet(url string) (string, error) {
	resp, err := http.Get(url)
	defer func() { _ = resp.Body.Close() }()

	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", errors.New(fmt.Sprintf("Error: %s returned %d",
			url, resp.StatusCode))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func getCurrentIP(checkIpUrl string) (string, error) {
	// Check the current IP
	body, err := doGet(checkIpUrl)
	if err != nil {
		return "", err
	}
	found := strings.Trim(re.FindString(body), " ")
	return found, nil
}

func ipHasChanged(current string, lastIp string) bool {
	return current != "" && (lastIp == "" || lastIp != current)
}

type UpdateParams struct {
	User       string
	ClientKey  string
	UpdateHost string
	HostName   string
	MyIp       string
}

func updateIP(params *UpdateParams) error {
	t, err := template.New("updateUrl").Parse("http://{{.User}}:{{.ClientKey}}" +
		"@{{.UpdateHost}}/v3/update?hostname={{.HostName}}&myip={{.MyIp}}")
	if err != nil {
		return err
	}
	var buf bytes.Buffer

	err = t.Execute(&buf, params)
	if err != nil {
		return err
	}

	var url = buf.String()
	body, err := doGet(url)
	if err != nil {
		return err
	}
	log.Println("Response body: ", body)
	return err
}

type UpdateConfig struct {
	User      string `yaml:"user"`
	ClientKey string `yaml:"clientkey"`
	HostName  string `yaml:"hostname"`
}

var updateHost = "members.dyndns.org"
var fileName = "last-ip.txt"
var checkIpUrl = "http://checkip.dyndns.com"

func readConfig(fileName string, config *UpdateConfig) error {
	yamlContent, err := ioutil.ReadFile(fileName)
	if err != nil {
		log.Fatalf("Error reading config file: %s\n", err)
		return err
	}

	err = yaml.UnmarshalStrict(yamlContent, config)
	if err != nil {
		log.Fatalf("Error reading config file: %s\n", err)
		return err
	}
	return nil
}

func main() {
	var config UpdateConfig
	err := readConfig("config.yaml", &config)
	if err != nil {
		log.Panic("Unable to read config file!  ", err)
		return
	}

	// Get the previous IP
	lastIp, err := readLastIP(fileName)
	if err != nil {
		log.Println("Unable to read last IP due to: ", err)
		log.Println("Continuing...")
	}

	current, err := getCurrentIP(checkIpUrl)
	if err != nil {
		log.Panic("Unable to get current IP due to: ", err)
		return
	}

	if !ipHasChanged(current, lastIp) {
		log.Println("IP unchanged.")
		return
	}

	log.Println("IP changed from ", lastIp, " to ", current)

	params := UpdateParams{
		User:       config.User,
		ClientKey:  config.ClientKey,
		UpdateHost: updateHost,
		HostName:   config.HostName,
		MyIp:       current,
	}

	err = updateIP(&params)
	if err != nil {
		log.Panic("Unable to update IP due to: ", err)
		return
	}

	err = storeCurrentIP(fileName, current)
	if err != nil {
		log.Panic("Unable to write IP to file due to: ", err)
		return
	}
}
