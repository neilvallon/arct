package main

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/peterh/liner"
	spin "github.com/tj/go-spin"
)

const baseURL = "http://web.archive.org/cdx/search/cdx?matchType=prefix&limit=25&filter=statuscode:200&collapse=urlkey&fl=original&url="

func FetchCompletions(rawPrefix string) []string {
	pref := url.QueryEscape(rawPrefix)

	res, err := http.Get(baseURL + pref)
	if err != nil {
		log.Println(err)
		return nil
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		log.Println(http.StatusText(res.StatusCode))
		return nil
	}

	var comps []string

	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		comps = append(comps, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Println(err)
		return nil
	}

	return comps
}

var memoCache = make(map[string][]string)

func SpinFetcher(rawPrefix string) []string {
	if c, ok := memoCache[rawPrefix]; ok {
		return c
	}

	ch := make(chan []string, 1)

	go func(rawPrefix string, ch chan []string) {
		ch <- FetchCompletions(rawPrefix)
	}(rawPrefix, ch)

	fmt.Printf("    ")
	defer fmt.Printf("\033[4D")

	s := spin.New()
	for i := 0; i < 30; i++ {
		select {
		case c := <-ch:
			if c != nil { // 0 len responses are fine
				memoCache[rawPrefix] = c
			}
			return c
		default:
			fmt.Printf("\033[36m\033[m %s ", s.Next())
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("\033[3D")
		}
	}
	return nil
}

func main() {
	line := liner.NewLiner()
	defer line.Close()

	line.SetCtrlCAborts(true)

	line.SetCompleter(SpinFetcher)

	for {
		if name, err := line.Prompt("> "); err == nil {
			line.AppendHistory(name)

			if c := FetchCompletions(name); 0 < len(c) {
				archURL := "http://web.archive.org/web/" + c[0]

				log.Printf("opening %q\n", archURL)

				var err error
				switch runtime.GOOS {
				case "linux":
					err = exec.Command("xdg-open", archURL).Run()
				case "darwin":
					err = exec.Command("open", archURL).Run()
				default:
					err = fmt.Errorf("unsupported platform")
				}

				if err != nil {
					log.Println("failed to open page in browser:", err)
				}
			} else {
				log.Println("could not find a page matching that url")
			}
		} else if err == liner.ErrPromptAborted {
			log.Print("Aborted")
			return
		} else {
			log.Print("Error reading line: ", err)
			return
		}
	}
}
