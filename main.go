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
	log "github.com/sirupsen/logrus"
)

type TorrentGetter struct {
	client              *torrent.Client
	client_mutex        sync.Mutex
	total_requests      sync.WaitGroup
	torrent_infos_mutex sync.Mutex
	torrent_infos       []TorrentInfo
	timeout             int64
}

type TorrentInfo struct {
	magnet string
	info   *tmetainfo.Info
}

func NewTorrentGetter(timeout int64) *TorrentGetter {
	c, _ := torrent.NewClient(nil)
	return &TorrentGetter{client: c, client_mutex: sync.Mutex{}, total_requests: sync.WaitGroup{}, torrent_infos_mutex: sync.Mutex{}, torrent_infos: []TorrentInfo{}, timeout: timeout}
}

func (tg *TorrentGetter) Close() {
	tg.client.Close()
}

func (tg *TorrentGetter) GetTorrentInfos() []TorrentInfo {
	tg.total_requests.Wait()
	return tg.torrent_infos
}

func (tg *TorrentGetter) getTorrentInfo(magnet string) *tmetainfo.Info {
	tg.total_requests.Add(1)
	defer tg.total_requests.Done()

	log.Info("Adding magnet: ", magnet)
	tg.client_mutex.Lock()
	t, err := tg.client.AddMagnet(magnet)
	tg.client_mutex.Unlock()

	if err != nil {
		log.Warning("Error adding magnet: ", err)
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

	if result != nil {
		tg.torrent_infos_mutex.Lock()
		tg.torrent_infos = append(tg.torrent_infos, TorrentInfo{magnet: magnet, info: result})
		tg.torrent_infos_mutex.Unlock()
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

func main() {
	magnets := flag.String("magnets", "", "File that contains the list of magnets to search")
	kw := flag.String("keywords", "", "List of keywords to search for in the torrent files")
	timeout := flag.Int("timeout", 120, "Timeout in seconds to wait for a response")
	verbose := flag.Bool("verbose", false, "Verbose output")
	flag.Parse()

	log.SetOutput(os.Stdout)
	log.SetLevel(log.WarnLevel)
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

		go tg.getTorrentInfo(line)
	}

	fmt.Println("Waiting for all requests to finish...")

	log.Info("Info: ", tg.GetTorrentInfos())

	// TODO: find how to sort this without being extremely annoying in go.
	for _, info := range tg.GetTorrentInfos() {
		maxFileSize := 0
		var largestFile []string

		for _, file := range info.info.Files {
			file := file
			if file.Length > int64(maxFileSize) {
				maxFileSize = int(file.Length)
				largestFile = file.Path
			}
		}

		fmt.Println("magnet:", info.magnet, largestFile, "with size", float32(maxFileSize)/1024./1024./1024., "GB")
	}

}
