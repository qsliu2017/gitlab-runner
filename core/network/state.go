package network

type UploadState int
type DownloadState int

const (
	UploadSucceeded UploadState = iota
	UploadTooLarge
	UploadForbidden
	UploadFailed
)

const (
	DownloadSucceeded DownloadState = iota
	DownloadForbidden
	DownloadFailed
	DownloadNotFound
)
