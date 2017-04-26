package main

import (
	"bufio"
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/Code-Hex/pget"
	"github.com/cheggaaa/pb"
)

// BufSize is size of buffer of a channel.
const BufSize = 1024

var proc int

// File is file object.
type File struct {
	SavePath string
	URL      string
	Err      error
}

func main() {

	var (
		input   string
		output  string
		isIndex bool
	)

	flag.StringVar(&input, "u", "", "List of url")
	flag.StringVar(&output, "o", "", "Output directory")
	flag.IntVar(&proc, "p", 1, "The number of goroutine")
	flag.BoolVar(&isIndex, "i", false, "Index file or finename")
	flag.Parse()

	// Set the number of goroutine
	runtime.GOMAXPROCS(runtime.NumCPU() - 1)

	// Set http timeout
	http.DefaultClient.Timeout = 30 * time.Second

	// Make channel for output
	resCh := make(chan *File, proc)
	defer close(resCh)

	// Initialize context
	ctx, cancel := context.WithCancel(context.Background())

	// Make channel for input
	chs := make([]chan *File, proc)

	// Execute queue.
	for i := 0; i < proc; i++ {
		ch := make(chan *File, BufSize)
		chs[i] = ch
		downloadQueue(ctx, ch, resCh)
	}

	defer func() {
		for i := range chs {
			close(chs[i])
		}
	}()

	defer cancel()

	// Open input file.
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
		chs[rtn] <- f
		idx++
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	bar := pb.StartNew(idx)

	finished := 0
	for res := range resCh {
		finished++
		if res.Err != nil {
			log.Println(res.Err)
		}
		if finished == idx {
			break
		}
		bar.Increment()
	}
	bar.Increment()
	bar.FinishPrint("Finished!")
}

func downloadQueue(ctx context.Context, fileCh chan *File, resCh chan *File) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case f := <-fileCh:
				f.Err = download(f)
				resCh <- f
			default:
			}
		}
	}()
}

func download(f *File) (err error) {

	if err = downloadWithPget(f); err == nil {
		return
	}
	log.Println(err)

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

func downloadWithPget(f *File) (err error) {
	p := pget.New()
	p.URLs = append(p.URLs, f.URL)
	dirName := filepath.Dir(f.SavePath)
	p.TargetDir = dirName
	fname := filepath.Base(f.SavePath)
	p.Utils.SetFileName(fname)
	if err = p.Checking(); err != nil {
		return
	}

	p.Procs = proc

	if err = p.Download(); err != nil {
		return
	}

	if err = p.Utils.BindwithFiles(proc); err != nil {
		return
	}
	return

}
