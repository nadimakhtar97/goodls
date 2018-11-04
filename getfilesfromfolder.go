// Package main (getfilesfromfolder.go) :
// These methods are for downloading all files from a shared folder of Google Drive.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	getfilelist "github.com/tanaikech/go-getfilelist"
	"google.golang.org/api/googleapi/transport"
)

const (
	driveAPI = "https://www.googleapis.com/drive/v3/files"
)

// mime2ext : Convert mimeType to extension.
func mime2ext(mime string) string {
	var obj map[string]interface{}
	json.Unmarshal([]byte(mimeVsEx), &obj)
	res, _ := obj[mime].(string)
	return res
}

// ext2mime : Convert extension to mimeType.
func ext2mime(ext string) string {
	var obj map[string]interface{}
	json.Unmarshal([]byte(extVsmime), &obj)
	res, _ := obj[ext].(string)
	return res
}

// downloadFileByAPIKey : Download file using API key.
func (p *para) downloadFileByAPIKey(file getfilelist.FileS) error {
	u, err := url.Parse(driveAPI)
	if err != nil {
		return err
	}
	u.Path = path.Join(u.Path, file.ID)
	q := u.Query()
	q.Set("key", p.APIKey)
	if strings.Contains(file.MimeType, "application/vnd.google-apps") {
		u.Path = path.Join(u.Path, "export")
		q.Set("mimeType", file.WebView)
	} else {
		q.Set("alt", "media")
	}
	u.RawQuery = q.Encode()
	bkWorkDir := p.WorkDir
	bkFilename := p.Filename
	p.WorkDir = file.WebLink
	p.Filename = file.Name
	timeOut, err := func(size int64, err error) (int64, error) {
		if err == nil || size == 0 {
			switch {
			case size < 100000000:
				return 3600, nil
			case size > 100000000:
				return 0, nil
			}
		}
		return 0, fmt.Errorf("%s", err)
	}(strconv.ParseInt(file.Size, 10, 64))
	if err != nil {
		return err
	}
	p.Client = &http.Client{
		Timeout: time.Duration(timeOut) * time.Second,
	}
	res, err := p.fetch(u.String())
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		r, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		return fmt.Errorf("%s", r)
	}
	p.saveFile(res)
	p.WorkDir = bkWorkDir
	p.Filename = bkFilename
	return nil
}

// makeFileByCondition : Make file by condition.
func (p *para) makeFileByCondition(file getfilelist.FileS) error {
	if er := chkFile(filepath.Join(file.WebLink, file.Name)); er {
		if !p.OverWrite && !p.Skip {
			return fmt.Errorf("'%s' is existing. If you want to overwrite, please use an option '--overwrite'", file.WebLink)
		}
		if p.OverWrite && !p.Skip {
			return p.downloadFileByAPIKey(file)
		}
		if !p.Disp && p.Skip {
			fmt.Printf("Downloading '%s' was skipped because of existing.\n", file.Name)
		}
	} else {
		return p.downloadFileByAPIKey(file)
	}
	return nil
}

// makeDir : Make a directory by checking duplication.
func (p *para) makeDir(folder string) error {
	if er := chkFile(folder); !er {
		if err := os.Mkdir(folder, 0777); err != nil {
			return err
		}
	} else {
		if !p.OverWrite && !p.Skip {
			return fmt.Errorf("'%s' is existing. If you want to overwrite, please use an option '--overwrite'", folder)
		}
	}
	return nil
}

// chkFile : Check the existence of file and directory in local PC.
func chkFile(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

// makeDirByCondition : Make directory by condition.
func (p *para) makeDirByCondition(dir string) error {
	var err error
	if er := chkFile(dir); er {
		if !p.OverWrite && !p.Skip {
			return fmt.Errorf("'%s' is existing. If you want to overwrite, please use option '--overwrite' or '--skip'", dir)
		}
		if p.OverWrite && !p.Skip {
			if err = p.makeDir(dir); err != nil {
				return err
			}
		}
		if !p.Disp && p.Skip {
			fmt.Printf("Creating '%s' was skipped because of existing.\n", dir)
		}
	} else {
		if err = p.makeDir(dir); err != nil {
			return err
		}
	}
	return nil
}

// initDownload : Download files by Drive API using API key.
func (p *para) initDownload(fileList *getfilelist.FileListDl) error {
	var err error
	if !p.Disp {
		fmt.Printf("Download files from a folder '%s'.\n", fileList.SearchedFolder.Name)
		fmt.Printf("There are %d files and %d folders in the folder.\n", fileList.TotalNumberOfFiles, fileList.TotalNumberOfFolders-1)
		fmt.Println("Starting download.")
	}
	idToName := map[string]interface{}{}
	for i, e := range fileList.FolderTree.Folders {
		idToName[e] = fileList.FolderTree.Names[i]
	}
	for _, e := range fileList.FileList {
		path := p.WorkDir
		for _, dir := range e.FolderTree {
			path = filepath.Join(path, idToName[dir].(string))
		}
		err = p.makeDirByCondition(path)
		if err != nil {
			return err
		}
		for _, file := range e.Files {
			if file.MimeType != "application/vnd.google-apps.script" {
				file.WebLink = path // Substituting
				size, _ := strconv.ParseInt(file.Size, 10, 64)
				p.Size = size
				err = p.makeFileByCondition(file)
				if err != nil {
					return err
				}
			} else {
				if !p.Disp {
					fmt.Printf("'%s' is a project file. Project file cannot be downloaded using API key.\n", file.Name)
				}
			}
		}
	}
	return nil
}

// defFormat : Default download format
func defFormat(mime string) string {
	var df map[string]interface{}
	json.Unmarshal([]byte(defaultformat), &df)
	dmime, _ := df[mime].(string)
	return dmime
}

// extToMime : Convert from extension to mimeType of the file on Local.
func extToMime(ext string) string {
	var fm map[string]interface{}
	json.Unmarshal([]byte(extVsmime), &fm)
	st, _ := fm[strings.Replace(strings.ToLower(ext), ".", "", 1)].(string)
	return st
}

// dupChkFoldersFiles : Check duplication of folder names and filenames.
func (p *para) dupChkFoldersFiles(fileList *getfilelist.FileListDl) {
	dupChk1 := map[string]bool{}
	cnt1 := 2
	for i, folderName := range fileList.FolderTree.Names {
		if !dupChk1[folderName] {
			dupChk1[folderName] = true
		} else {
			fileList.FolderTree.Names[i] = folderName + "_" + strconv.Itoa(cnt1)
		}
	}
	extt := strings.ToLower(p.Ext)
	for i, list := range fileList.FileList {
		if len(list.Files) > 0 {
			dupChk2 := map[string]bool{}
			cnt2 := 2
			for j, file := range list.Files {
				if !dupChk2[file.Name] {
					dupChk2[file.Name] = true
				} else {
					ext := filepath.Ext(file.Name)
					if ext != "" {
						fileList.FileList[i].Files[j].Name = file.Name[0:len(file.Name)-len(ext)] + "_" + strconv.Itoa(cnt2) + ext
					} else {
						fileList.FileList[i].Files[j].Name = file.Name + "_" + strconv.Itoa(cnt2)
					}
					cnt2++
				}
				mime := defFormat(file.MimeType)
				if extt != "" {
					if mime != "" {
						cmime := func() string {
							if (extt == "txt" || extt == "text") && file.MimeType == "application/vnd.google-apps.spreadsheet" {
								return extToMime("csv")
							} else if extt == "zip" && file.MimeType == "application/vnd.google-apps.presentation" {
								return extToMime("pptx")
							}
							return extToMime(extt)
						}()
						if cmime != "" {
							fileList.FileList[i].Files[j].WebView = cmime // Substituting as OutMimeType
						} else {
							fileList.FileList[i].Files[j].WebView = mime // Substituting as OutMimeType
						}
					}
				} else {
					fileList.FileList[i].Files[j].WebView = mime // Substituting as OutMimeType
				}
				if file.MimeType != "application/vnd.google-apps.script" {
					ext := filepath.Ext(file.Name)
					if ext == "" {
						fileList.FileList[i].Files[j].Name += mime2ext(fileList.FileList[i].Files[j].WebView)
					}
				}
			}
		}
	}
}

// getFilesFromFolder: This method is the main method for downloading all files in a shread folder.
func (p *para) getFilesFromFolder() error {
	client := &http.Client{
		Transport: &transport.APIKey{Key: p.APIKey},
	}
	fileList, err := getfilelist.Folder(p.SearchID).Do(client)
	if err != nil {
		return err
	}
	if p.ShowFileInf {
		r, _ := json.Marshal(fileList)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", r)
		return nil
	}
	p.dupChkFoldersFiles(fileList)
	if err := p.initDownload(fileList); err != nil {
		return err
	}
	return nil
}