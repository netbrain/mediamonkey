package main

import (
	"flag"
	"fmt"
	"github.com/xor-gate/goexif2/exif"
	"gopkg.in/djherbis/times.v1"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var src = flag.String("src","", "src to recursively scan for files")
var dst = flag.String("dst", "", "dst to copy files to")

func main() {
	flag.Parse()

	if *src == "" || *dst == ""{
		flag.PrintDefaults()
		return
	}

	err := filepath.Walk(*src, func(path string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir(){
			log.Printf("found directory %s",path)
			return nil
		}
		log.Printf("found file %s",path)

		date,err := dateAndTime(path)
		if err != nil {
			return err
		}
		log.Printf("guessed date %s on %s",date.Format("2006/01/02"),path)

		return copy(path,fmt.Sprintf("%s%s",filepath.Join(*dst,date.Format("2006/01/02150405")),strings.ToLower(filepath.Ext(path))))
	})
	if err != nil {
		log.Fatal(err)
	}
}

func copy(src,dst string) error{
	info,err := os.Stat(src)
	if err != nil{
		return err
	}
	dstInfo,err  := os.Stat(dst)
	if err == nil {
		if dstInfo.Size() != info.Size() {
			log.Printf("%s already exists, but integrity check failed",dst)
			if err := os.Remove(dst); err != nil {
				return err
			}
			return copy(src,dst)
		}
		log.Printf("%s already exists, no copy required",dst)
		return nil
	}

	if !os.IsNotExist(err){
		return err
	}

	log.Printf("reading file %s",src)
	input, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}


	if err := os.MkdirAll(filepath.Dir(dst),0744); err != nil {
		return err
	}

	log.Printf("copying %s to %s",src,dst)
	err = ioutil.WriteFile(dst, input, 0644)
	if err != nil {
		return err
	}

	return os.Chtimes(dst,time.Now(),info.ModTime())
}

func dateAndTime(path string) (time.Time,error) {
	f,err := os.Open(path)
	if err != nil {
		return time.Time{},err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		log.Printf("could not detect exif date/time: %s",err)
	}else {
		date,err := x.DateTime()
		if err == nil {
			return date,nil
		}
	}

	ts,err := times.Stat(path)
	if err != nil {
		return time.Time{},err
	}
	if ts.HasBirthTime(){
		return ts.BirthTime(),nil
	}
	return ts.ModTime(),nil
}