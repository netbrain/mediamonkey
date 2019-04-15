package main

import (
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/mostlygeek/go-exiftool"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var src = flag.String("src", "", "src to recursively scan for files")
var dst = flag.String("dst", "", "dst to copy files to")

var threads = flag.Int("workers", runtime.NumCPU(), "number of worker threads")
var wg sync.WaitGroup
var workChan = make(chan func() error)

var pool *exiftool.Pool

var exifFlags = []string{
	"-j",
	"-DateAquired",
	"-CreateDate",
	"-DateTime",
	"-DateTimeOriginal",
	"-ModifyDate",
	"-FileModifyDate",
}

func main() {
	flag.Parse()

	var err error
	pool, err = exiftool.NewPool("exiftool", *threads, exifFlags...)
	if err != nil {
		log.Fatal(err)
	}

	if *src == "" || *dst == "" {
		flag.PrintDefaults()
		return
	}

	for i := 0; i < *threads; i++ {
		go func() {
			for f := range workChan {
				err := f()
				if err != nil {
					log.Fatal(err)
				}
				wg.Done()
			}
		}()
	}

	_ = filepath.Walk(*src, func(path string, info os.FileInfo, err error) error {
		wg.Add(1)
		workChan <- work(path, info)
		return nil
	})

	wg.Wait()

}

func work(path string, info os.FileInfo) func() error {
	return func() error {
		if info == nil || info.IsDir() {
			log.Printf("found directory %s", path)
			return nil
		}
		log.Printf("found file %s", path)

		date, err := dateAndTime(path)
		if err != nil {
			return err
		}
		log.Printf("guessed date %s on %s", date.Format("2006/01/02"), path)

		return copy(path, *dst, date)
	}

}

func copy(src, dstPath string, date time.Time) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	log.Printf("reading file %s", src)
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	dstFile := fmt.Sprintf("%s_%x%s", filepath.Join(dstPath, date.Format("2006/01/02150405")), md5.Sum(input),strings.ToLower(filepath.Ext(src)))

	dstInfo, err := os.Stat(dstFile)
	if err == nil {
		if dstInfo.Size() != info.Size() {
			log.Printf("%s already exists, but integrity check failed", dstFile)
			if err := os.Remove(dstFile); err != nil {
				return err
			}
			return copy(src, dstPath, date)
		}
		log.Printf("%s already exists, no copy required", dstFile)
		return nil
	}

	if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dstFile), 0755); err != nil {
		return err
	}

	log.Printf("copying %s to %s", src, dstFile)
	err = ioutil.WriteFile(dstFile, input, 0644)
	if err != nil {
		return err
	}

	return os.Chtimes(dstFile, time.Now(), info.ModTime())
}

type exifdateset struct {
	SourceFile     string
	CreateDate     string
	ModifyDate     string
	FileModifyDate string
}

func (e exifdateset) Time() (time.Time, error) {
	var t string
	if e.CreateDate != "" {
		t = e.CreateDate
	} else if e.ModifyDate != "" {
		t = e.ModifyDate
	} else {
		t = e.FileModifyDate
	}

	for _, l := range []string{"2006:01:02 15:04:05Z07:00", "2006:01:02 15:04:05"} {
		parsed, err := time.Parse(l, t)
		if err != nil {
			continue
		}
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("could not parse time %s", t)
}

func dateAndTime(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	data, err := pool.Extract(path)
	if err != nil {
		log.Printf("could not detect exiftool tags: %s", err)
	}

	var dates []exifdateset
	if err := json.Unmarshal(data, &dates); err != nil {
		return time.Time{}, err
	}
	return dates[0].Time()
}
