package main

import (
	"encoding/xml"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gorilla/mux"
)

type CertificationTest struct {
	XMLName       xml.Name `xml:"certification-test"`
	RHCertVersion string   `xml:"rhcert-version,attr"`
	RHCertRelease string   `xml:"rhcert-release,attr"`
	Hardware      Hardware `xml:"hardware"`
	Output        string   `xml:",innerxml"`
}

type Hardware struct {
	Release string `xml:"release"`
	OS      OS     `xml:"os"`
	Model   string `xml:"model"`
	Make    string `xml:"make"`
	Vendor  string `xml:"vendor"`
}

type OS struct {
	Release string `xml:"release"`
	Product string `xml:"product"`
}

var tmpl = template.Must(template.ParseFiles(filepath.Join("templates", "index.html")))

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/upload", uploadHandler).Methods("POST")
	http.Handle("/", r)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Starting server on: 8088")
	http.ListenAndServe(":8088", nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.Execute(w, nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(100 << 20)
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to retrieve file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	tempDir := "uploads"
	err = os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		http.Error(w, "Unable to create temporary directory", http.StatusInternalServerError)
		return
	}
	tempFile, err := os.CreateTemp(tempDir, "upload-*.xml")
	if err != nil {
		http.Error(w, "Unable to create temporary file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Unable to copy file content", http.StatusInternalServerError)
		return
	}

	fileBytes, err := os.ReadFile(tempFile.Name())
	if err != nil {
		http.Error(w, "Unable to read file", http.StatusInternalServerError)
		return
	}

	var certificationTest CertificationTest
	err = xml.Unmarshal(fileBytes, &certificationTest)
	if err != nil {
		http.Error(w, "Unable to parse XML", http.StatusInternalServerError)
		return
	}

	output := strings.TrimSpace(certificationTest.Output)

	kdumpConfig := extractSection(output, "kdump configuration:", "updated kdump configuration")
	updatedKdumpConfig := extractSection(output, "updated kdump configuration:", "restarting kdump with new configuration..")

	vmcore := extractSection(output, "Looking for vmcore image", "/output&gt;")
	errorRegex := regexp.MustCompile(`Error: could not locate vmcore file`)
	vmcoreStatus := errorRegex.FindStringSubmatch(vmcore)

	var finalVmcoreStatus string

	if len(vmcoreStatus) > 0 {
		finalVmcoreStatus = vmcoreStatus[0]
	} else {
		foundKdumpRegex := regexp.MustCompile(`Found kdump image:\s*(.*)`)
		foundKdump := foundKdumpRegex.FindStringSubmatch(vmcore)

		if len(foundKdump) > 0 {
			finalVmcoreStatus = foundKdump[0]
		} else {
			finalVmcoreStatus = "Vmcore status not found"
		}
	}

	systemctlStatus := extractSection(output, "Checking kdump service", "Crash recovery kernel arming")
	re := regexp.MustCompile(`Active:\s*(\w+)`)
	match := re.FindStringSubmatch(systemctlStatus)
	messageStatus := extractSection(output, `<message level="FAIL">`, "</message>")

	// Debug print
	fmt.Println("KdumpConfig:", kdumpConfig)
	fmt.Println("UpdatedKdumpConfig:", updatedKdumpConfig)
	fmt.Println("Vmcore status:", finalVmcoreStatus)
	fmt.Println("SystemctlStatus:", systemctlStatus)

	if len(match) > 1 {
		fmt.Printf("systemctl status kdump: %s\n", match[1])
	} else {
		fmt.Println("Status not found")
	}
	fmt.Println("Error Log:", messageStatus)

	data := struct {
		KernelRelease      string
		ProductRhel        string
		RHELVersion        string
		RhcertVersion      string
		KdumpConfig        string
		UpdatedKdumpConfig string
		VmcoreStatus       string
		SystemctlStatus    string
		Error              string
	}{
		KernelRelease:      certificationTest.Hardware.Release,
		ProductRhel:        certificationTest.Hardware.OS.Product,
		RHELVersion:        certificationTest.Hardware.OS.Release,
		RhcertVersion:      certificationTest.RHCertVersion,
		KdumpConfig:        kdumpConfig,
		UpdatedKdumpConfig: updatedKdumpConfig,
		VmcoreStatus:       finalVmcoreStatus,
		SystemctlStatus:    match[1],
		Error:              messageStatus,
	}

	tmpl.Execute(w, data)
}

func extractSection(content, startMarker, endMarker string) string {
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return ""
	}
	section := content[startIdx+len(startMarker):]
	endIdx := strings.Index(section, endMarker)
	if endIdx == -1 {
		section = strings.TrimSpace(section)
	} else {
		section = strings.TrimSpace(section[:endIdx])
	}

	return html.UnescapeString(section)
}
