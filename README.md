---

# Kraken

Kraken is a lightweight web-based tool built in Go that allows users to upload an XML file and extract essential system information directly on their screen.

## Features

- **Web Server in Go**: Kraken is powered by a web server written in Go, ensuring high performance and simplicity.
- **Easy XML Upload**: Users can easily upload their result XML files through a user-friendly interface.
- **Data Extraction**: Upon uploading, Kraken extracts and displays critical information from the XML file, including:
  - **Kernel Release**
  - **Operating System**
  - **RHEL Version**
  - **Test Suite Version**
  - **Kdump Configuration** and **Updated Kdump Configuration**

## Getting Started

### Prerequisites

- Go (version 1.16 or higher)
- Git (for cloning the repository)

### Installation

1. Clone the repository:

   ```bash
   git clone git@github.com:Red-Hat-Partner-CertOps/kraken.git  
   cd kraken
   ```

2. Run the Go web server:

   ```bash
   go run main.go
   ```

3. Open your web browser and navigate to `http://localhost:8088`.

### Usage

1. On the Kraken web interface, click on "choose file" button.
2. Select the XML file containing the system information.
3. Once the file is uploaded, Kraken will automatically extract and display the following details:
   - **Kernel Release**
   - **Operating System**
   - **RHEL Version**
   - **Test Suite Version**
   - **Kdump Configuration** and **Updated Kdump Configuration**

## Contributing

We welcome contributions! If you'd like to contribute to Kraken, please fork the repository and create a pull request with your changes.

## License

This project is licensed under the Apache License - see the [LICENSE](LICENSE) file for details.

---

Feel free to modify any part of this README to better fit your needs!
