package phs3minio

import (
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
)

// BuildDownloadRequest builds a download/presigned request from object, ids, and optional args.
// args[0]: path string; args[1]: bucket string; args[2]: expires int (>0).
func BuildDownloadRequest(object string, userID, merchantID int64, args ...interface{}) DownloadRequest {
	req := DownloadRequest{
		Object:     object,
		UserID:     userID,
		MerchantID: merchantID,
	}
	if len(args) >= 1 {
		if v, ok := args[0].(string); ok && v != "" {
			req.Path = v
		}
		if len(args) >= 2 {
			if v, ok := args[1].(string); ok && v != "" {
				req.Bucket = v
			}
			if len(args) >= 3 {
				if v, ok := args[2].(int); ok && v > 0 {
					req.Expires = int32(v)
				}
			}
		}
	}
	return req
}

// BuildUploadRequestForMultipart builds an upload request from a multipart file.
// args[0]: bucket string; args[1]: expires int (>0) as uint32.
func BuildUploadRequestForMultipart(userID, merchantID int64, path string, file multipart.File, fileHeader *multipart.FileHeader, args ...interface{}) (UploadRequest, error) {
	req := UploadRequest{
		Filename:    fileHeader.Filename,
		Size:        uint64(fileHeader.Size),
		ContentType: fileHeader.Header.Get("Content-Type"),
		Path:        path,
		UserID:      userID,
		MerchantID:  merchantID,
	}
	if len(args) >= 1 {
		if v, ok := args[0].(string); ok && v != "" {
			req.Bucket = v
		}
		if len(args) >= 2 {
			if v, ok := args[1].(int); ok && v > 0 {
				req.Expires = uint32(v)
			}
		}
	}
	buf := make([]byte, int(fileHeader.Size))
	n, err := io.ReadFull(file, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		if err == io.EOF && fileHeader.Size == 0 {
			req.Content = buf[:n]
			return req, nil
		}
		return req, err
	}
	req.Content = buf[:n]
	if len(req.Content) < int(fileHeader.Size) {
		rest, err2 := io.ReadAll(file)
		if err2 != nil {
			return req, err2
		}
		req.Content = append(req.Content, rest...)
	}
	if req.ContentType == "" {
		req.ContentType = mime.TypeByExtension(filepath.Ext(req.Filename))
	}
	if req.Size == 0 && len(req.Content) > 0 {
		req.Size = uint64(len(req.Content))
	}
	return req, nil
}

// BuildUploadRequestForFile reads a file from disk and builds an upload request.
// Optional args use the same convention as legacy merchant services: first string
// at index 0 is bucket; int at index 1 is expires.
func BuildUploadRequestForFile(userID, merchantID int64, path, fileLocation string, args ...interface{}) (UploadRequest, error) {
	req := UploadRequest{
		Path:       path,
		UserID:     userID,
		MerchantID: merchantID,
	}
	for i, arg := range args {
		switch v := arg.(type) {
		case string:
			if i == 0 && v != "" {
				req.Bucket = v
			}
		case int:
			if i == 1 && v > 0 {
				req.Expires = uint32(v)
			}
		}
	}
	f, err := os.Open(fileLocation)
	if err != nil {
		return req, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return req, err
	}
	sz := st.Size()
	buf := make([]byte, int(sz))
	if _, err = io.ReadFull(f, buf); err != nil {
		return req, err
	}
	req.Content = buf
	req.Filename = f.Name()
	req.Size = uint64(sz)
	req.ContentType = mime.TypeByExtension(filepath.Ext(fileLocation))
	return req, nil
}
