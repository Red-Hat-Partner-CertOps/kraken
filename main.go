package main

import (
	"encoding/xml"
	"fmt"
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

var tmpl = template.Must(template.ParseFiles(
	filepath.Join("templates", "upload.html"),
	filepath.Join("templates", "upload-result.html"),
	filepath.Join("templates", "home.html"),
))

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/upload", uploadHandler).Methods("GET", "POST")
	http.Handle("/", r)

	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	fmt.Println("Starting server on: 8088")
	http.ListenAndServe(":8088", nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "home.html", nil)
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		tmpl.ExecuteTemplate(w, "upload.html", nil)
		return
	}

	if r.Method == http.MethodPost {
		// Handle file upload and processing
		if err := r.ParseMultipartForm(100 << 20); err != nil {
			http.Error(w, "Unable to parse form", http.StatusBadRequest)
			return
		}

		file, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Unable to retrieve file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		fileBytes, err := readFile(file)
		if err != nil {
			http.Error(w, "Unable to read file", http.StatusInternalServerError)
			return
		}

		certTest, err := parseCertificationTest(fileBytes)
		if err != nil {
			http.Error(w, "Unable to parse XML", http.StatusInternalServerError)
			return
		}

		// Extract information and check conditions
		data := processCertificationData(certTest)

		tmpl.ExecuteTemplate(w, "upload-result.html", data)
	}
}

func readFile(file io.Reader) ([]byte, error) {
	tempDir := "uploads"
	err := os.MkdirAll(tempDir, os.ModePerm)
	if err != nil {
		return nil, fmt.Errorf("unable to create temporary directory: %v", err)
	}
	tempFile, err := os.CreateTemp(tempDir, "upload-*.xml")
	if err != nil {
		return nil, fmt.Errorf("unable to create temporary file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, file)
	if err != nil {
		return nil, fmt.Errorf("unable to copy file content: %v", err)
	}

	return os.ReadFile(tempFile.Name())
}

func parseCertificationTest(fileBytes []byte) (*CertificationTest, error) {
	var certTest CertificationTest
	err := xml.Unmarshal(fileBytes, &certTest)
	if err != nil {
		return nil, err
	}
	return &certTest, nil
}

func processCertificationData(certTest *CertificationTest) map[string]string {
	output := strings.TrimSpace(certTest.Output)

	kernelDebugInfo := extractSection(output, `<command command="rpm -q kernel-debuginfo" return-value="0" signal="0">`, "</command>")
	kernelDebugVersion := extractSection(kernelDebugInfo, "<stdout>", "</stdout>")
	kdumpConfig := extractSection(output, "kdump configuration:", "stderr")
	updatedKdumpConfig := extractSection(output, "updated kdump configuration:", "restarting kdump with new configuration..")
	vmcoreStatus := checkVmcoreStatus(output)
	systemctlStatus := getSystemctlStatus(output)

	// Debug utility check based on hardware release and kernel debug version
	debugUtilityCheck := checkDebugUtility(certTest.Hardware.Release, kernelDebugVersion)

	// Recommended solution URL based on kernel release, RHEL version, and vmcore status
	recommendedSolution := getRecommendedSolution(certTest.Hardware.Release, certTest.Hardware.OS.Release, vmcoreStatus)

	return map[string]string{
		"KernelRelease":       certTest.Hardware.Release,
		"ProductRhel":         certTest.Hardware.OS.Product,
		"RHELVersion":         certTest.Hardware.OS.Release,
		"RhcertVersion":       certTest.RHCertVersion,
		"KernelDebugVersion":  kernelDebugVersion,
		"KdumpConfig":         kdumpConfig,
		"UpdatedKdumpConfig":  updatedKdumpConfig,
		"VmcoreStatus":        vmcoreStatus,
		"SystemctlStatus":     systemctlStatus,
		"DebugUtilityCheck":   debugUtilityCheck,
		"RecommendedSolution": recommendedSolution,
	}
}

func getSystemctlStatus(output string) string {
	systemctlOutput := extractSection(output, "systemctl status kdump", "</command>")
	activeRegex := regexp.MustCompile(`Active:\s*(\w+)`)
	match := activeRegex.FindStringSubmatch(systemctlOutput)
	if len(match) > 1 {
		return match[1] // Return only the "active" part
	}
	return "Status not found"
}

func checkDebugUtility(kernelRelease, kernelDebugVersion string) string {
	if kernelRelease == kernelDebugVersion {
		return "The kernel-debuginfo utility and kernel version match."
	}
	return "The kernel-debuginfo utility and kernel version do not match."
}

func getRecommendedSolution(kernelRelease, rhelVersion, vmcoreStatus string) string {
	if kernelRelease == "5.14.0-427.13.1.el9_4.x86_64" && rhelVersion == "9.4" && vmcoreStatus == "could not locate vmcore file" {
		return "https://access.redhat.com/solutions/7068656"
	}
	return ""
}

func checkVmcoreStatus(output string) string {
	vmcore := extractSection(output, "Looking for vmcore image", "/output&gt;")
	errorRegex := regexp.MustCompile(`Error: could not locate vmcore file`)
	if errorRegex.MatchString(vmcore) {
		return "could not locate vmcore file"
	}
	foundKdumpRegex := regexp.MustCompile(`Found kdump image:\s*(.*)`)
	foundKdump := foundKdumpRegex.FindStringSubmatch(vmcore)
	if len(foundKdump) > 0 {
		return foundKdump[0]
	}
	return "Vmcore status not found"
}

func extractSection(content, startMarker, endMarker string) string {
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return ""
	}
	section := content[startIdx+len(startMarker):]
	endIdx := strings.Index(section, endMarker)
	if endIdx == -1 {
		return strings.TrimSpace(section)
	}
	return strings.TrimSpace(section[:endIdx])
}
