package main

import (
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

type CertificationTest struct {
	XMLName       xml.Name `xml:"certification-test"`
	RHCertVersion string   `xml:"rhcert-version,attr"`
	RHCertRelease string   `xml:"rhcert-release,attr"`
	Hardware      Hardware `xml:"hardware"`
	Message       Message  `xml:"message"`
	Test          Test     `xml:"test"`
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

type Message struct {
	Level string `xml:"level"`
}

type Test struct {
	Name string `xml:"name"`
}

var tmpl = template.Must(template.ParseFiles("upload.html"))

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/upload", uploadHandler).Methods("POST")
	http.Handle("/", r)

	fmt.Println("Starting server on: 8088")
	http.ListenAndServe(":8088", nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.Execute(w, nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
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

	kdumpConfig := extractSection(output, "kdump configuration:", "updated kdump configuration:")
	updatedKdumpConfig := extractSection(output, "updated kdump configuration:", "systemctl status kdump:")
	systemctlStatus := extractSection(output, "systemctl status kdump:", "</message>")

	// Debug print
	fmt.Println("TestName:", certificationTest.Test.Name)
	fmt.Println("TestStatus:", certificationTest.Message.Level)
	fmt.Println("KdumpConfig:", kdumpConfig)
	fmt.Println("UpdatedKdumpConfig:", updatedKdumpConfig)
	fmt.Println("SystemctlStatus:", systemctlStatus)

	data := struct {
		KernelRelease      string
		ProductRhel        string
		RHELVersion        string
		RhcertVersion      string
		TestName           string
		TestStatus         string
		KdumpConfig        string
		UpdatedKdumpConfig string
		SystemctlStatus    string
	}{
		KernelRelease:      certificationTest.Hardware.Release,
		ProductRhel:        certificationTest.Hardware.OS.Product,
		RHELVersion:        certificationTest.Hardware.OS.Release,
		RhcertVersion:      certificationTest.RHCertVersion,
		TestName:           certificationTest.Test.Name,
		TestStatus:         certificationTest.Message.Level,
		KdumpConfig:        kdumpConfig,
		UpdatedKdumpConfig: updatedKdumpConfig,
		SystemctlStatus:    systemctlStatus,
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
		return section
	}
	return strings.TrimSpace(section[:endIdx])
}