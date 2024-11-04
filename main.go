package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	title         = "### 2CH THREAD PARSER ###"
	urlPattern    = `^(https?|http)://[^\s/$.?#].[^\s]*$`
	re            = regexp.MustCompile(urlPattern)
	client        = http.Client{Timeout: 30 * time.Second}
	urlFileName   = "urls.txt"
	maxConcurrent = 5
	retries       = 5
)

func main() {
	fmt.Println(title)
	taskCh := make(chan string, maxConcurrent)
	beforeAll := time.Now()

	data, err := createConfigFile()
	if err != nil {
		fmt.Println(err)
		return
	}

	urlList := createUrlList(data)

	var urlWg sync.WaitGroup
	for _, site := range urlList {
		urlWg.Add(1)
		go func(site string) {
			defer urlWg.Done()
			for k := range collectUrls(site) {
				taskCh <- k
			}
		}(site)
	}

	go func() {
		urlWg.Wait()
		close(taskCh)
	}()

	var workerWg sync.WaitGroup
	for i := 0; i < maxConcurrent; i++ {
		workerWg.Add(1)
		go worker(taskCh, &workerWg)
	}

	workerWg.Wait()
	fmt.Println(maxConcurrent, "workers:", time.Since(beforeAll))
	showAlert()
}

func worker(ch chan string, wg *sync.WaitGroup) {
	for url := range ch {
		if err := downloadFile(url); err != nil {
			fmt.Printf("Downloading err: %v\n", err)

			if err = retryDownload(url); err != nil {
				fmt.Println(err)
				return
			}
		}
	}
	defer wg.Done()
}

func retryDownload(url string) error {
	for i := 1; i <= retries; i++ {
		fmt.Printf("Retry download #%v - %v\n", i, url)
		if err := downloadFile(url); err == nil {
			fmt.Printf("Success retry download - %v\n", url)
			return nil
		}
	}
	return fmt.Errorf("failed to download %v", url)
}

func collectUrls(url string) map[string]struct{} {
	resp, err := client.Get(url)

	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	urlPrefix := "https://2ch.hk"
	urlOfFiles := map[string]struct{}{}

	re := regexp.MustCompile(`href="(.*/src/\d+/\d+\.[^"]+)"`)
	matches := re.FindAllStringSubmatch(string(body), -1)

	for _, match := range matches {
		value := urlPrefix + match[1]
		if _, ok := urlOfFiles[value]; !ok {
			urlOfFiles[value] = struct{}{}
		}
	}

	if len(urlOfFiles) == 0 {
		fmt.Println("map of URLs is empty")
		return nil
	}

	return urlOfFiles
}

func createConfigFile() ([]byte, error) {
	if _, err := os.Stat(urlFileName); !os.IsNotExist(err) {
		data, err := os.ReadFile(urlFileName)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("%v file is empty", urlFileName)
		}

		return data, nil
	}

	file, err := os.Create(urlFileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return nil, fmt.Errorf("%v file is not found and was created", urlFileName)
}

func createUrlList(data []byte) []string {
	urlList := []string{}
	lines := strings.Split(string(data), "\r\n")

	for _, line := range lines {
		if isUrl(line) {
			urlList = append(urlList, line)
		} else {
			fmt.Println(line, "is not valid URL!")
		}
	}

	if len(urlList) == 0 {
		fmt.Println("config file doensn't conatain any valid URL")
		return nil
	}
	return urlList
}

func isUrl(userUrl string) bool {
	return re.MatchString(userUrl)
}

func getFilename(url string) string {
	tmp := strings.Split(url, "/")
	return tmp[len(tmp)-1]
}

func getDirName(url string) string {
	tmp := strings.Split(url, "/")
	return fmt.Sprintf("%v_%v", tmp[len(tmp)-2], tmp[len(tmp)-4])
}

func showAlert() {
	fmt.Print("Press Enter to exit...")
	fmt.Scanln()
}

func downloadFile(url string) error {
	dirname := getDirName(url)
	filePath := fmt.Sprintf("%s/%s", dirname, getFilename(url))

	if _, err := os.Stat(dirname); os.IsNotExist(err) {
		if err := os.MkdirAll(dirname, os.ModePerm); err != nil {
			return err
		}
	}

	if _, err := os.Stat(filePath); err == nil {
		return nil
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}

	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	fmt.Printf("File was downloaded: %v\n", url)
	return nil
}
