package crocmobile

import (
	"context"
	"errors"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/schollz/croc/v9/src/croc"
	"github.com/schollz/croc/v9/src/models"
	"github.com/schollz/croc/v9/src/utils"
)

type Handlers interface {
	TransferStarted(requestID int)
	TransferProgress(
		requestID int,
		fileSize,
		bytesTransferred,
		msElapsed,
		totalBytesTransferred,
		totalMsElapsed int64)
	TransferComplete(requestID int, err error)
}

type Options struct {
    IsSender        bool
    SharedSecret    string
    Debug           bool
	DebugWrapper    bool
    RelayAddress    string
    RelayAddress6   string
    RelayPorts      string
    RelayPassword   string
    Stdout          bool
    NoPrompt        bool
    NoMultiplexing  bool
    DisableLocal    bool
    OnlyLocal       bool
    IgnoreStdin     bool
    Ask             bool
    SendingText     bool
    NoCompress      bool
    IP              string
    Overwrite       bool
    Curve           string
    HashAlgorithm   string
    ThrottleUpload  string
    ZipFolder       bool
}

type TransferRequest struct {
	ID			int
	Type		int
	ctxCancel	*context.CancelFunc
}

func (r *TransferRequest) Cancel() {
	(*r.ctxCancel)()
}

const (
	SendTx		int = 0
	ReceieveTx	int = 1
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func NewOptions() *Options {
    return &Options {
        SharedSecret:    utils.GetRandomName(),
        IsSender:        true,
        Debug:           false,
        NoPrompt:        false,
        RelayAddress:    "croc.schollz.com",
        RelayAddress6:   "croc6.schollz.com",
        Stdout:          false,
        DisableLocal:    false,
        OnlyLocal:       false,
        IgnoreStdin:     false,
        RelayPorts:      "9009,9010,9011,9012,9013",
        Ask:             false,
        NoMultiplexing:  false,
        RelayPassword:   "pass123",
        SendingText:     false,
        NoCompress:      false,
        Overwrite:       false,
        Curve:           "p256",
        HashAlgorithm:   "xxhash",
        ThrottleUpload:  "",
        ZipFolder:       false,
        DebugWrapper:    false,
    }
}

func Send(fileAtPath string, options *Options, handlers Handlers) (request *TransferRequest, err error) {
    if len(fileAtPath) == 0 {
		err = errors.New("fileAtPath is not set")
        return
    }
	if options == nil {
		options = NewOptions()
	}
    if len(options.SharedSecret) == 0 {
		err = errors.New("shared secret is not set")
        return
    }

    crocOptions := croc.Options{
        SharedSecret:   options.SharedSecret,
        IsSender:       options.IsSender,
        Debug:          options.Debug,
        NoPrompt:       options.NoPrompt,
        RelayAddress:   options.RelayAddress,
        RelayAddress6:  options.RelayAddress6,
        Stdout:         options.Stdout,
        DisableLocal:   options.DisableLocal,
        OnlyLocal:      options.OnlyLocal,
        IgnoreStdin:    options.IgnoreStdin,
        RelayPorts:     strings.Split(options.RelayPorts, ","),
        Ask:            options.Ask,
        NoMultiplexing: options.NoMultiplexing,
        RelayPassword:  options.RelayPassword,
        SendingText:    options.SendingText,
        NoCompress:     options.NoCompress,
        Overwrite:      options.Overwrite,
        Curve:          options.Curve,
        HashAlgorithm:  options.HashAlgorithm,
        ThrottleUpload: options.ThrottleUpload,
        ZipFolder:      options.ZipFolder,
    }

    if crocOptions.RelayAddress != models.DEFAULT_RELAY {
        crocOptions.RelayAddress6 = ""
    } else if crocOptions.RelayAddress6 != models.DEFAULT_RELAY6 {
        crocOptions.RelayAddress = ""
    }

	// var fnames []string
    fnames := []string{fileAtPath}

    minimalFileInfos, emptyFoldersToTransfer, totalNumberFolders, err := croc.GetFilesInfo(fnames, crocOptions.ZipFolder)
    if err != nil {
        return
    }

    crocClient, err := croc.New(crocOptions)
    if err != nil {
        return
	}

	ctx, request := newTransferRequest(SendTx)
	crocClient.Context = ctx

	if handlers != nil {
		crocClient.Handlers = croc.Handlers{
			TransferStarted: func ()  {
				handlers.TransferStarted(request.ID)
			},
			TransferProgress: func(fileSize, bytesTransferred, msElapsed, totalBytesTransferred, totalMsElapsed int64) {
				handlers.TransferProgress(request.ID, fileSize, bytesTransferred, msElapsed, totalBytesTransferred, totalMsElapsed)
			},
		}
	}

    go func() {
        if !options.DebugWrapper {
            // silence croc library output
            old := os.Stderr
            r, w, _ := os.Pipe()
            os.Stderr = w
            defer func() {
                w.Close()
                r.Close()
                os.Stderr = old
            }()
        }
        err := crocClient.Send(minimalFileInfos, emptyFoldersToTransfer, totalNumberFolders)
		if handlers != nil {
			handlers.TransferComplete(request.ID, err)
		}
		(*request.ctxCancel)()
    }()

    return
}

func newTransferRequest(transferType int) (ctx context.Context, request *TransferRequest) {
	id := rand.Intn(10000)	// 0-9999
	ctx, ctxCancel := context.WithCancel(context.Background())
	request = &TransferRequest{
		ID: id,
		Type: transferType,
		ctxCancel: &ctxCancel,
	}
	return
}
