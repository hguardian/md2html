package main

import (
	"encoding/base64"
	"fmt"
	"github.com/jessevdk/go-flags"
	"github.com/russross/blackfriday"
	"io/ioutil"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

type Options struct {
	InputFile  string `long:"input" short:"i" description:"input Markdown"`
	OutputFile string `long:"output" short:"o" description:"output HTML"`
	EmbedImage bool   `long:"embed" short:"e" description:"embed image by base64 encoding"`
}

const (
	template = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>%s</title>
%s
</head>
<body>
<div class="markdown-body">%s</div>
</body>
</html>`

	extensions = blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS
)

func main() {
	var opts Options
	inputs, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}

	if len(opts.InputFile) > 0 {
		inputs = []string{opts.InputFile}
	}

	if len(inputs) <= 0 {
		fmt.Fprintln(os.Stderr, "Please specify input Markdown")
		os.Exit(1)
	}

	for _, input := range inputs {
		files, err := filepath.Glob(input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if len(files) <= 0 {
			fmt.Fprintln(os.Stderr, "File is not found")
			os.Exit(1)
		}

		if len(opts.OutputFile) > 0 {
			if err := writeHtmlConcat(files, opts.OutputFile, opts.EmbedImage); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		} else {
			for _, file := range files {
				if err := writeHtml(file, file+".html", opts.EmbedImage); err != nil {
					fmt.Fprintln(os.Stderr, err)
				}
			}
		}
	}
}

func writeHtml(input, output string, embed bool) error {
	fi, err := os.Open(input)
	if err != nil {
		return err
	}
	defer fi.Close()

	md, err := ioutil.ReadAll(fi)
	if err != nil {
		return err
	}

	js := string(js_bytes[:len(js_bytes)])
	css := string(css_bytes[:len(css_bytes)])
	html := string(blackfriday.MarkdownCommon(md))

	if embed {
		html, err = embedImage(html, filepath.Dir(input))
		if err != nil {
			return err
		}
	}

	fo, err := os.Create(output)
	if err != nil {
		return err
	}
	defer fo.Close()

	fmt.Fprintf(fo, template, input, js+"\n"+css, html)
	return nil
}

func writeHtmlConcat(inputs []string, output string, embed bool) error {
	js := string(js_bytes[:len(js_bytes)])
	css := string(css_bytes[:len(css_bytes)])
	html := ""

	for _, input := range inputs {
		fi, err := os.Open(input)
		if err != nil {
			return err
		}
		defer fi.Close()

		md, err := ioutil.ReadAll(fi)
		if err != nil {
			return err
		}

		h := string(blackfriday.MarkdownCommon(md))

		if embed {
			h, err = embedImage(h, filepath.Dir(input))
			if err != nil {
				return err
			}
		}

		html += h
	}

	fo, err := os.Create(output)
	if err != nil {
		return err
	}
	defer fo.Close()

	re := regexp.MustCompile(filepath.Ext(output) + "$")
	title := filepath.Base(re.ReplaceAllString(output, ""))
	fmt.Fprintf(fo, template, title, js+"\n"+css, html)
	return nil
}

func embedImage(src, parent string) (string, error) {
	re_find, err := regexp.Compile(`(<img[\S\s]+?src=")([\S\s]+?)("[\S\s]+?/>)`)
	if err != nil {
		return src, err
	}
	img_tags := re_find.FindAllString(src, -1)

	dest := src
	for _, t := range img_tags {
		img_src := re_find.ReplaceAllString(t, "$2")
		img_path := img_src
		if !filepath.IsAbs(img_src) {
			img_path = filepath.Join(parent, img_src)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
		}

		f, err := os.Open(img_path)
		if err != nil {
			pathErr := err.(*os.PathError)
			errno := pathErr.Err.(syscall.Errno)
			if errno != 0x7B { // suppress ERROR_INVALID_NAME
				fmt.Fprintln(os.Stderr, err)
			}
			continue
		}
		defer f.Close()

		d, err := ioutil.ReadAll(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}

		b64img := base64.StdEncoding.EncodeToString(d)
		re_replace, err := regexp.Compile(`(<img[\S\s]+?src=")` + regexp.QuoteMeta(img_src) + `("[\S\s]+?/>)`)
		if err != nil {
			return src, err
		}

		ext := filepath.Ext(img_src)
		mime_type := mime.TypeByExtension(ext)
		if len(mime_type) <= 0 {
			mime_type = "image"
		}
		dest = re_replace.ReplaceAllString(dest, "${1}data:"+mime_type+";base64,"+b64img+"${2}")
	}
	return dest, nil
}
