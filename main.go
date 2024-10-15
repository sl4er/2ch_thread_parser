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
	pattern       = `^(https?|http)://[^\s/$.?#].[^\s]*$`
	re            = regexp.MustCompile(pattern)
	maxConcurrent = 10
)

func main() {
	fmt.Println(title)
	taskCh := make(chan string, maxConcurrent)
	beforeAll := time.Now()

	readUrls, err := createConfigFile()
	if err != nil {
		fmt.Println(err)
		return
	}

	var urlWg sync.WaitGroup
	for _, site := range readUrls {
		urlWg.Add(1)
		go func(site string) {
			defer urlWg.Done()
			for _, v := range collectUrls(site) {
				taskCh <- v
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

func collectUrls(url string) []string {
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
	urlOfFiles := []string{}

	re := regexp.MustCompile(`href="(/fag/src/\d+/\d+\.[^"]+)"`)
	matches := re.FindAllStringSubmatch(string(body), -1)

	for _, match := range matches {
		value := urlPrefix + match[1]
		urlOfFiles = append(urlOfFiles, value)
	}

	return urlOfFiles
}

func createConfigFile() ([]string, error) {
	configName := "urls.txt"
	if _, err := os.Stat(configName); !os.IsNotExist(err) {
		data, err := os.ReadFile(configName)
		if err != nil {
			log.Fatal(err)
		}
		if len(data) == 0 {
			fmt.Println("Add urls into config file -", configName, "!!!")
			return nil, err
		} 

		urlList := createUrlList(data)
		return urlList, nil
		
	} else {
		fmt.Println("Config file with urls is not found and was created!")
		file, err := os.Create(configName)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
	}

	return nil, nil
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
	larr := len(tmp) - 1
	return tmp[larr]
}

func getDirName(url string) string {
	tmp := strings.Split(url, "/")
	larr := len(tmp) - 2
	subarr := len(tmp) - 4
	r := tmp[subarr] + "_" + tmp[larr]
	return r
}

func showAlert() {
	fmt.Print("Press Enter to exit...")
	fmt.Scanln()
}


func downloadFile(url string) error {
	firstUrl := url
	dirname := getDirName(firstUrl)
	filename := getFilename(firstUrl)
	filePath := fmt.Sprintf("%s/%s", dirname, filename)

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
