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

// BufSize is size of buffer of a channel.
const BufSize = 1024

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

	errCh := make(chan error, proc)
	defer close(errCh)
	ctx, cancel := context.WithCancel(context.Background())

	chs := make([]chan *File, proc)

	for i := 0; i < proc; i++ {
		ch := make(chan *File, BufSize)
		chs[i] = ch
		downloadQueue(ctx, ch, errCh)
	}

	defer func() {
		for i := range chs {
			close(chs[i])
		}
	}()

	defer cancel()

	fp, err := os.Open(input)
	if err != nil {
		log.Fatal(err)
	}
	defer fp.Close()
	scanner := bufio.NewScanner(fp)
	idx := 0
	for scanner.Scan() {
		u := scanner.Text()
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
		rtn := idx % proc
		fmt.Printf("Downloading: %s on proc %d\n", u, rtn)
		chs[rtn] <- f
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
	fmt.Println("Finished.")
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
