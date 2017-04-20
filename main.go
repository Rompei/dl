package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// File is file object.
type File struct {
	SavePath string
	URL      string
}

func main() {

	var (
		input   string
		output  string
		proc    int
		isIndex bool
	)

	flag.StringVar(&input, "u", "", "List of url")
	flag.StringVar(&output, "o", "", "Output directory")
	flag.IntVar(&proc, "p", 1, "The number of goroutine")
	flag.BoolVar(&isIndex, "i", false, "Index file or finename")
	flag.Parse()

	// Setting timeout.
	http.DefaultClient.Timeout = 30 * time.Second

	fileCh := make(chan *File, proc)
	errCh := make(chan error, proc)
	defer close(fileCh)
	defer close(errCh)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < proc; i++ {
		downloadQueue(ctx, fileCh, errCh)
	}

	fp, err := os.Open(input)
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()
	scanner := bufio.NewScanner(fp)
	idx := 0
	for scanner.Scan() {
		u := scanner.Text()
		fmt.Printf("\rDownloading: %s", u)
		var f *File
		if isIndex {
			ext := filepath.Ext(u)
			f = &File{
				SavePath: filepath.Join(output, strconv.Itoa(idx)+ext),
				URL:      u,
			}
		} else {
			fname := filepath.Base(u)
			f = &File{
				SavePath: filepath.Join(output, fname),
				URL:      u,
			}
		}
		fileCh <- f
		idx++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Println()

	finished := 0
	for err := range errCh {
		finished++
		if err != nil {
			log.Println(err)
		}
		if finished == idx {
			break
		}
	}
}

func downloadQueue(ctx context.Context, fileCh chan *File, errCh chan error) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case f := <-fileCh:
				if err := download(f); err != nil {
					errCh <- err
					continue
				}
				errCh <- nil
			default:
			}
		}
	}()
}

func download(f *File) (err error) {
	res, err := http.Get(f.URL)
	if err != nil {
		return
	}
	defer res.Body.Close()
	dst, err := os.Create(f.SavePath)
	if err != nil {
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, res.Body)
	if err != nil {
		return
	}
	return
}
