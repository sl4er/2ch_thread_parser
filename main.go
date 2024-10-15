package main

import (
	"fmt"
	"io"
	"log"
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
	urlFileName   = "urls.txt"
	maxConcurrent = 5
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

	var wg sync.WaitGroup
	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go worker(taskCh, &wg)
	}

	wg.Wait()
	fmt.Println(maxConcurrent, "workers:", time.Since(beforeAll))
	defer showAlert()
}

func worker(ch <-chan string, wg *sync.WaitGroup) {
	for url := range ch {
		if err := downloadFile(url); err != nil {
			fmt.Printf("Downloading err: %v\n", err)
		}
		fmt.Printf("File was downloaded: %v\n", url)
	}
	defer wg.Done()
}

func collectUrls(url string) map[string]struct{} {
	resp, err := http.Get(url)
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

	re := regexp.MustCompile(`href="(/fag/src/\d+/\d+\.[^"]+)"`)
	matches := re.FindAllStringSubmatch(string(body), -1)

	for _, match := range matches {
		value := urlPrefix + match[1]
		if _, ok := urlOfFiles[value]; !ok {
			urlOfFiles[value] = struct{}{}
		}
	}

	return urlOfFiles
}

func createConfigFile() ([]byte, error) {
	if _, err := os.Stat(urlFileName); !os.IsNotExist(err) {
		data, err := os.ReadFile(urlFileName)
		if err != nil {
			log.Fatal(err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("%v file is empty", urlFileName)
		}

		return data, nil
	}

	file, err := os.Create(urlFileName)
	if err != nil {
		log.Fatal(err)
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

	resp, err := http.Get(url)
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

	return nil
}
