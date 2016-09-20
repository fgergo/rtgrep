package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	
	"github.com/nilium/glob"
)

func main() {
	duration := flag.Duration("timeout", 2000*time.Millisecond, "timeout in milliseconds")
	path := flag.String("path", ".", "path to start from")
	filepattern := flag.String("filepattern", "*", "file name pattern")
	flag.Usage = func() {
		fmt.Printf("%s recursively almost-greps until timeout. pattern is checked byte for byte. Original: bketelsen.\n", os.Args[0])
		fmt.Printf("Usage: %v [flags] pattern\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(-1)
	}
	pattern := flag.Arg(0)
	ctx, _ := context.WithTimeout(context.Background(), *duration)
	m, err := search(ctx, *path, pattern,  *filepattern)
	if err != nil {
		log.Fatal(err)
	}
	for _, name := range m {
		fmt.Println(name)
	}
	fmt.Println(len(m), "hits")
}

func search(ctx context.Context, root string, pattern string, filepattern string) ([]string, error) {
	g, ctx := errgroup.WithContext(ctx)
	paths := make(chan string, 100)
	// get all the paths

	g.Go(func() error {
		defer close(paths)

		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			ok, err := glob.Matches(glob.PatternStr(filepattern), info.Name()) 
			if err != nil {
				return nil
			}
			if !info.IsDir() && !ok{
				return nil
			}

			select {
			case paths <- path:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})

	})

	c := make(chan string, 100)
	for path := range paths {
		p := path
		g.Go(func() error {
			data, err := ioutil.ReadFile(p)
			if err != nil {
				return err
			}
			if !bytes.Contains(data, []byte(pattern)) {
				return nil
			}
			select {
			case c <- p:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		})
	}
	go func() {
		g.Wait()
		close(c)
	}()

	var m []string
	for r := range c {
		m = append(m, r)
	}
	return m, g.Wait()
}
