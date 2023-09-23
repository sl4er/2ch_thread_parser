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

	"github.com/gocolly/colly"
)

func main() {
	fmt.Println("### 2CH THREAD PARSER ###")
	maxConcurrent := 5 // Максимальное количество одновременных загрузок
	var wg sync.WaitGroup
	taskCh := make(chan string, maxConcurrent)

	readUrls, _ := createConfigFile()
	urlArray := [][]string{}
	for _, url := range readUrls {
		urlArray = append(urlArray, collectUrls(url))
	}
	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go worker(taskCh, &wg)
	}

	// Передача задач в канал
	for _, site := range urlArray {
		for _, url := range site {
			taskCh <- url
		}
		fmt.Println("All files in", getThreadName(site[0]), "were downloaded")
	}

	close(taskCh) // Закрытие канала, чтобы завершить горутины-рабочие группы
	wg.Wait()     // Ожидаем завершения всех горутин-рабочих групп
	defer showAlert()
}

func worker(ch <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	for url := range ch {
		if err := downloadFile(url); err != nil {
			fmt.Printf("Ошибка при загрузке: %v\n", err)
		}
	}
}

func createConfigFile() ([]string, error) {
	configName := "urls.txt"
	urlsList := []string{}
	if _, err := os.Stat(configName); err == nil {
		data, err := os.ReadFile(configName)
		if err != nil {
			log.Fatal(err)
		}
		if len(data) == 0 {
			fmt.Println("Add urls into config file -", configName, "!!!")
		} else {
			t := strings.Split(string(data), "\r\n")
			for _, i := range t {
				if isUrl(i) {
					urlsList = append(urlsList, i)
				} else {
					fmt.Println(i, "is not valid URL!")
				}
			}
		}
	} else {
		fmt.Println("Config file with urls is not found and was created!")
		file, err := os.Create(configName)
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
	}

	return urlsList, nil
}

func collectUrls(url string) []string {
	urlPrefix := "https://2ch.hk"

	c := colly.NewCollector(
		colly.AllowedDomains("2ch.hk"),
	)
	urlOfFiles := []string{}

	c.OnHTML("img", func(e *colly.HTMLElement) {
		src := e.Attr("data-src")
		if !strings.HasPrefix(src, "/stickers/") && len(src) != 0 {
			value := urlPrefix + src
			urlOfFiles = append(urlOfFiles, value)
		}
	})
	c.Visit(url)
	return urlOfFiles
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

func getThreadName(site string) string {
	r := strings.Split(site, "/")
	thread := strings.Join(r[:len(r)-1], "/")
	return thread
}

func showAlert() {
	fmt.Print("Press Enter to exit...")
	fmt.Scanln()
}

func isUrl(userUrl string) bool {
	pattern := `^(https?|http)://[^\s/$.?#].[^\s]*$`
	re := regexp.MustCompile(pattern)
	return re.MatchString(userUrl)
}

func downloadFile(url string) error {
	firstUrl := url
	dirname := getDirName(firstUrl)
	filename := getFilename(firstUrl)
	filePath := fmt.Sprintf("%s/%s", dirname, filename)

	// Проверяем, существует ли директория, и создаем ее, если она не существует
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
