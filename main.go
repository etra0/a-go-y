package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	tmetainfo "github.com/anacrolix/torrent/metainfo"
	color "github.com/fatih/color"
	colorable "github.com/mattn/go-colorable"
	log "github.com/sirupsen/logrus"
)

type TorrentGetter struct {
	client       *torrent.Client
	client_mutex sync.Mutex
	magnets      []string
	torrents     chan *TorrentInfo
	timeout      int64
}

type TorrentInfo struct {
	magnet string
	info   *tmetainfo.Info
}

func NewTorrentGetter(timeout int64) *TorrentGetter {
	c, _ := torrent.NewClient(nil)
	return &TorrentGetter{
		client:       c,
		client_mutex: sync.Mutex{},
		magnets:      []string{},
		timeout:      timeout,
		torrents:     make(chan *TorrentInfo),
	}
}

func (tg *TorrentGetter) AddRequest(magnet string) {
	tg.magnets = append(tg.magnets, magnet)
}

func (tg *TorrentGetter) Close() {
	tg.client.Close()
}

func (tg *TorrentGetter) DoTorrentRequests() {
	wg := sync.WaitGroup{}
	for _, magnet := range tg.magnets {
		magnet := magnet
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := tg.getTorrentInfo(magnet)
			if result != nil {
				tg.torrents <- &TorrentInfo{magnet: magnet, info: result}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(tg.torrents)
	}()
}

func (tg *TorrentGetter) GetTorrentInfos() <-chan *TorrentInfo {
	return tg.torrents
}

func (tg *TorrentGetter) getTorrentInfo(magnet string) *tmetainfo.Info {
	log.Info("Adding magnet: ", magnet)
	tg.client_mutex.Lock()
	t, err := tg.client.AddMagnet(magnet)
	tg.client_mutex.Unlock()

	if err != nil {
		log.Warning("Error adding magnet: ", magnet)
		return nil
	}

	var result *tmetainfo.Info
	select {
	case <-t.GotInfo():
		result = t.Info()
	case <-time.After(time.Duration(tg.timeout) * time.Second):
		log.Info("Timed out on ", magnet)
		result = nil
	}

	return result
}

func containsAllKeywords(line string, keywords []string) bool {
	for _, keyword := range keywords {
		lowerLine := strings.ToLower(line)
		lowerKeyword := strings.ToLower(keyword)
		if !strings.Contains(lowerLine, lowerKeyword) {
			return false
		}
	}

	return true
}

func formatTorrent(info *TorrentInfo) {
	maxFileSize := 0
	var largestFile string

	for _, file := range info.info.Files {
		file := file
		if file.Length > int64(maxFileSize) {
			maxFileSize = int(file.Length)
			largestFile = file.Path[len(file.Path)-1]
		}
	}

	title_print := color.New(color.Bold, color.FgGreen)
	title_print.Printf("* Magnet: ")
	fmt.Printf("%s\n", info.magnet)

	fileSizeInGb := float32(maxFileSize) / 1024. / 1024. / 1024.

	label := color.New(color.Bold)
	label.Printf("* Largest file: ")
	fmt.Printf("%s\n", largestFile)
	label.Printf("* Largest file size: ")
	fmt.Printf("%.2f GB\n", fileSizeInGb)
	fmt.Println()
}

func main() {
	// Log initialization
	log.SetLevel(log.PanicLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true})
	log.SetOutput(colorable.NewColorableStdout())

	magnets := flag.String("magnets", "", "File that contains the list of magnets to search")
	kw := flag.String("keywords", "", "List of keywords to search for in the torrent files")
	timeout := flag.Int("timeout", 120, "Timeout in seconds to wait for a response")
	verbose := flag.Bool("verbose", false, "Verbose output")
	flag.Parse()

	if *verbose {
		log.SetLevel(log.InfoLevel)
	}

	if *magnets == "" || *kw == "" {
		log.Fatal("You need to pass both a magnet file and a keyword list")
	}

	keywords := strings.Split(*kw, " ")

	f, err := os.Open(*magnets)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	tg := NewTorrentGetter(int64(*timeout))
	defer tg.Close()

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)

		// Check that the magnet contains *all* the keywords in lowercase.
		// I know, this is awful, but it's a quick hack.
		if !containsAllKeywords(line, keywords) {
			continue
		}

		go tg.AddRequest(line)
	}
	fmt.Println("Found", len(tg.magnets), "magnets")

	tg.DoTorrentRequests()
	fmt.Println("Waiting for all requests to finish...")

	// TODO: find how to sort this without being extremely annoying in go.
	for info := range tg.GetTorrentInfos() {
		formatTorrent(info)
	}

}
