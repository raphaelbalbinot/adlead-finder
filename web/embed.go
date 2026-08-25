package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// Assets armazena os arquivos estáticos do frontend embutidos no binário
//
//go:embed static/*
var Assets embed.FS

// GetFileSystem retorna o sub-filesystem de static/ para servir via http.FileServer
func GetFileSystem() (http.FileSystem, error) {
	sub, err := fs.Sub(Assets, "static")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}
