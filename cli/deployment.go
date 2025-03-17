package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Deployment struct {
	dir               string
	name              string
	blaxelDeployments []Result
	dockerfile        string
	zip               *os.File
	cwd               string
	r                 *Operations
}

func (d *Deployment) Generate() error {
	if d.name == "" {
		split := strings.Split(d.cwd, "/")
		d.name = split[len(split)-1]
	}

	// Generate the blaxel deployment yaml
	d.blaxelDeployments = []Result{
		{
			ApiVersion: "blaxel.ai/v1alpha1",
			Kind:       "Agent",
			Metadata: map[string]interface{}{
				"name": d.name,
				"labels": map[string]string{
					"x-blaxel-auto-generated": "true",
				},
			},
			Spec: map[string]interface{}{
				"model": d.name,
			},
		},
	}
	rootCommand, err := findTSRootCmdAsString(false)
	if err != nil {
		return fmt.Errorf("failed to find root command: %w", err)
	}
	packageManagerCommand, err := findTSPackageManagerCommandAsString(true)
	if err != nil {
		return fmt.Errorf("failed to find package manager command: %w", err)
	}
	// Generate the dockerfile
	d.dockerfile = fmt.Sprintf(`
FROM node:22-alpine
WORKDIR /blaxel
COPY package.json /blaxel/package.json
RUN %s
COPY . .

ENTRYPOINT ["%s"]	
`, strings.Join(packageManagerCommand, " "), strings.Join(rootCommand, "\",\""))

	// Zip the directory
	err = d.Zip()
	if err != nil {
		return fmt.Errorf("failed to zip file: %w", err)
	}

	return nil
}

func (d *Deployment) Apply() error {
	blaxelDir := filepath.Join(d.cwd, ".blaxel")
	if _, err := os.Stat(blaxelDir); err == nil {
		fmt.Println("Applying additional resources from .blaxel directory...")
		_, err = d.r.Apply(blaxelDir, WithRecursive(true))
		if err != nil {
			return fmt.Errorf("failed to apply .blaxel directory: %w", err)
		}
	}
	applyResults, err := d.r.ApplyResources(d.blaxelDeployments)
	if err != nil {
		return fmt.Errorf("failed to apply agent deployment: %w", err)
	}

	for _, result := range applyResults {
		if result.Result.UploadURL != "" {
			err := d.Upload(result.Result.UploadURL)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}
		}
	}

	return nil
}

func (d *Deployment) Upload(url string) error {
	// Open the zip file
	zipFile, err := os.Open(d.zip.Name())
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer zipFile.Close()

	// Get the file size
	fileInfo, err := zipFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Create a new HTTP request
	req, err := http.NewRequest("PUT", url, zipFile)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set the content length
	req.ContentLength = fileInfo.Size()

	// Set the content type to application/zip
	req.Header.Set("Content-Type", "application/zip")

	// Perform the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}
	defer resp.Body.Close()

	// Check the response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	fmt.Println("Upload successful")
	return nil
}

func (d *Deployment) Zip() error {
	ignoredPaths := []string{
		".blaxel",
		".git",
		"node_modules",
	}
	zipFile, err := os.CreateTemp("", ".blaxel.zip")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add Dockerfile to the zip
	dockerfileHeader := &zip.FileHeader{
		Name:   "Dockerfile",
		Method: zip.Deflate,
	}
	dockerfileWriter, err := zipWriter.CreateHeader(dockerfileHeader)
	if err != nil {
		return fmt.Errorf("failed to create Dockerfile in zip: %w", err)
	}
	_, err = dockerfileWriter.Write([]byte(d.dockerfile))
	if err != nil {
		return fmt.Errorf("failed to write Dockerfile to zip: %w", err)
	}

	err = filepath.Walk(d.cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		for _, ignoredPath := range ignoredPaths {
			if strings.HasPrefix(path, filepath.Join(d.cwd, ignoredPath)) {
				return nil
			}
		}
		if path == d.cwd {
			return nil
		}

		// Create a header based on the file info
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Set the header name to the relative path
		header.Name, err = filepath.Rel(d.cwd, path)
		if err != nil {
			return err
		}

		// If it's a directory, we need to add a trailing slash
		if info.IsDir() {
			header.Name += "/"
		} else {
			header.Method = zip.Deflate
		}

		// Create a writer for the file
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a file, write its content to the zip
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to zip directory: %w", err)
	}

	d.zip = zipFile

	return nil
}

func (d *Deployment) Print() error {
	for _, deployment := range d.blaxelDeployments {
		fmt.Println(deployment)
	}
	fmt.Println("---")
	fmt.Println(d.dockerfile)
	fmt.Println("---")
	d.PrintZip()
	return nil
}

func (d *Deployment) PrintZip() error {
	// Reopen the file to get the reader
	zipFile, err := os.Open(d.zip.Name())
	if err != nil {
		return fmt.Errorf("failed to reopen zip file: %w", err)
	}

	// Get the file size
	fileInfo, err := zipFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Print the content of the zip file
	zipReader, err := zip.NewReader(zipFile, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %w", err)
	}

	for _, file := range zipReader.File {
		fmt.Printf("File: %s, Size: %d bytes\n", file.Name, file.FileInfo().Size())
	}

	return nil
}
