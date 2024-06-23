package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	VERSION            = "0.0.1"
	CATBOX_HOST string = "https://catbox.moe/user/api.php"
)

var (
	CATBOX_USER_HASH string
	CATBOX_ALBUM     string
	SAVE_LOCATION    string
	USE_CATBOX       bool = true
)

func printUsage() {
	fmt.Print("savemgr v" + VERSION + "\n" +
		"Usage:\n" +
		"\tsavemgr <application>\n" +
		"\n" +
		"\tThere should be a .savemgr file in the working directory when this process is launched.\n" +
		"\tAll args will be used to spawn a process using those.\n" +
		"\n" +
		"Configuration file variables:\n" +
		"\tCATBOX_USER_HASH - User hash so the album can be editted.\n" +
		"\tCATBOX_ALBUM - Album where the game saves are being stored.\n" +
		"\tSAVE_LOCATION - Folder where game saves are located.\n",
	)
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	config, err := os.Open(".savemgr")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open config file: %s\n.", err.Error())
		return
	}

	fmt.Println("Opening config file.")
	parseConfig(config)

	if USE_CATBOX {
		fmt.Println("Checking online last save.")
		getLatestSave()
	}

	fmt.Println("Starting process.")
	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	proc, err := os.StartProcess(os.Args[1], os.Args[2:], &procAttr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start process: %s.\n", err.Error())
		return
	}

	proc.Wait()

	now := time.Now()
	zip_name := fmt.Sprintf("save.%d%02d%02d_%02d%02d", now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute())

	func() {
		save_folder := os.DirFS(SAVE_LOCATION)
		zip_file, err := os.Create(zip_name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create zip with the saves: %s.\n", err.Error())
			os.Exit(1)
		}
		defer zip_file.Close()
		w := zip.NewWriter(zip_file)
		defer w.Close()
		err = w.AddFS(save_folder)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write to zip with the saves: %s.\n", err.Error())
			os.Exit(1)
		}
	}()

	uploadFile(zip_name)
}

func parseConfig(file *os.File) {
	m := map[string]*string{
		"CATBOX_USER_HASH": &CATBOX_USER_HASH,
		"CATBOX_ALBUM":     &CATBOX_ALBUM,
		"SAVE_LOCATION":    &SAVE_LOCATION,
	}

	s := bufio.NewScanner(file)
	for s.Scan() {
		l := s.Text()
		equal_p := strings.Index(l, "=")
		if equal_p < 0 {
			continue
		}
		k := l[:equal_p]
		k = strings.TrimSpace(k)
		if p, ok := m[k]; ok {
			v := l[equal_p+1:]
			*p = strings.TrimSpace(v)
		} else {
			fmt.Fprintf(os.Stderr, "Ignoring key %s given in the config file.\n", k)
		}
	}

	if SAVE_LOCATION == "" {
		fmt.Fprintln(os.Stderr, "One of the necessary variables wasnt defined. Exiting.")
		os.Exit(1)
	}
	if CATBOX_USER_HASH == "" || CATBOX_ALBUM == "" {
		fmt.Println("Saving files locally only.")
		USE_CATBOX = false
	}
}

func getLatestSave() {
	resp, err := http.Get("https://catbox.moe/c/" + CATBOX_ALBUM)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch saves in the catbox album: %s.\n", err.Error())
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch saves in the catbox album: %s.\n", err.Error())
		os.Exit(1)
	}
	s := string(body)
	reg := regexp.MustCompile("target='_blank'>(https://files.catbox.moe/.{18,23})</a>")
	links := reg.FindAllStringSubmatch(s, -1)
	if len(links) == 0 {
		fmt.Println("No previous save online.")
		return
	}

	lsl := links[len(links)-1][1]
	file_ext_pos := strings.LastIndexByte(lsl, '.')
	date := lsl[file_ext_pos+1:]
	save_filename := "save." + date
	if _, err := os.Stat(save_filename); !os.IsNotExist(err) {
		fmt.Println("Latest save is already downloaded.")
		return
	}

	func() {
		save_dl_req, err := http.NewRequest("GET", lsl, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download the latest save: %s.\n", err.Error())
			os.Exit(1)
		}
		save_dl_req.Header.Add("User-Agent", "curl/8.6.0")

		client := &http.Client{}
		save_dl_resp, err := client.Do(save_dl_req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to download the latest save: %s.\n", err.Error())
			os.Exit(1)
		}
		defer save_dl_resp.Body.Close()

		save, err := os.Create(save_filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create zip with the newest save: %s.\n", err.Error())
			os.Exit(1)
		}
		defer save.Close()
		io.Copy(save, save_dl_resp.Body)
	}()

	if SAVE_LOCATION[len(SAVE_LOCATION)-1] == '/' {
		SAVE_LOCATION = SAVE_LOCATION[:len(SAVE_LOCATION)-1]
	}
	backup_path := SAVE_LOCATION + "_savemgr_bu"
	if _, err := os.Stat(SAVE_LOCATION); !os.IsNotExist(err) {
		err := os.Rename(SAVE_LOCATION, backup_path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create to backup current using save: %s.\n", err.Error())
			os.Exit(1)
		}
	}
	err = os.MkdirAll(SAVE_LOCATION, os.ModePerm)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create to backup current using save: %s.\n", err.Error())
		os.Exit(1)
	}

	handle_err := func(err error) {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to extract latest save zip: %s.\n", err.Error())
			os.Rename(backup_path, SAVE_LOCATION)
			os.Exit(1)
		}
	}
	save_reader, err := zip.OpenReader(save_filename)
	handle_err(err)
	defer save_reader.Close()

	for _, f := range save_reader.File {
		path := fmt.Sprintf("%s/%s", SAVE_LOCATION, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, os.ModePerm)
			continue
		}
		p := strings.LastIndexByte(f.Name, '/')
		if p > 0 {
			os.MkdirAll(fmt.Sprintf("%s/%s", SAVE_LOCATION, f.Name[:p]), os.ModePerm)
		}
		w, err := os.Create(path)
		handle_err(err)
		r, err := f.Open()
		handle_err(err)
		_, err = io.Copy(w, r)
		handle_err(err)
		w.Close()
		r.Close()
	}
	os.RemoveAll(backup_path)
}

func uploadFile(zip_name string) {
	file, err := os.Open(zip_name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open zip file to upload: %s.\n", err.Error())
		os.Exit(1)
	}
	defer file.Close()

	r, w := io.Pipe()
	m := multipart.NewWriter(w)

	go func() {
		defer w.Close()
		defer m.Close()

		m.WriteField("reqtype", "fileupload")
		m.WriteField("userhash", CATBOX_USER_HASH)
		part, err := m.CreateFormFile("fileToUpload", zip_name)
		if err != nil {
			return
		}
		io.Copy(part, file)
	}()

	var newfilename string
	client := &http.Client{}
	func() {
		req, _ := http.NewRequest(http.MethodPost, CATBOX_HOST, r)
		req.Header.Add("Content-Type", m.FormDataContentType())

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to open zip file to upload: %s.\n", err.Error())
			os.Exit(1)
		}
		defer resp.Body.Close()

		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "File uploaded, but failed to add to the album: %s.", err.Error())
			os.Exit(1)
		}
		link := string(bytes)
		file_pos := strings.LastIndexByte(link, '/')
		newfilename = link[file_pos+1:]
	}()

	r, w = io.Pipe()
	m = multipart.NewWriter(w)
	go func() {
		defer w.Close()
		defer m.Close()

		m.WriteField("reqtype", "addtoalbum")
		m.WriteField("userhash", CATBOX_USER_HASH)
		m.WriteField("short", CATBOX_ALBUM)
		m.WriteField("files", newfilename)
	}()

	req, _ := http.NewRequest(http.MethodPost, CATBOX_HOST, r)
	req.Header.Add("Content-Type", m.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to add file to album: %s.\n", err.Error())
		os.Exit(1)
	}
	resp.Body.Close()
}
