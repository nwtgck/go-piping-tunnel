// TODO: should send fin to notify finish
package pmux

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/backoff"
	"github.com/nwtgck/go-piping-tunnel/crypto_duplex"
	"github.com/nwtgck/go-piping-tunnel/early_piping_duplex"
	"github.com/nwtgck/go-piping-tunnel/hb_duplex"
	"github.com/nwtgck/go-piping-tunnel/openpgp_duplex"
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

type server struct {
	httpClient      *http.Client
	headers         []piping_util.KeyValue
	baseUploadUrl   string
	baseDownloadUrl string
	enableHb        bool
	encrypts        bool
	passphrase      string
	cipherType      string // NOTE: encryption in pmux can be updated in the different way in the future such as negotiating algorithm
}

type client struct {
	httpClient      *http.Client
	headers         []piping_util.KeyValue
	baseUploadUrl   string
	baseDownloadUrl string
	enableHb        bool
	encrypts        bool
	passphrase      string
	cipherType      string
}

type serverConfigJson struct {
	Hb bool `json:"hb"`
}

type syncJson struct {
	SubPath string `json:"sub_path"`
}

const pmuxVersion uint32 = 1
const pmuxMimeType = "application/pmux"
const httpTimeout = 50 * time.Second

var pmuxVersionBytes [4]byte
var IncompatiblePmuxVersion = errors.Errorf("incompatible pmux version, expected %d", pmuxVersion)
var NonPmuxMimeTypeError = errors.Errorf("invalid content-type, expected %s", pmuxMimeType)
var IncompatibleServerConfigError = errors.Errorf("imcompatible server config")
var DifferentHbSettingError = errors.Errorf("different hb setting from server's")

func init() {
	binary.BigEndian.PutUint32(pmuxVersionBytes[:], pmuxVersion)
}

func headersWithPmux(headers []piping_util.KeyValue) []piping_util.KeyValue {
	return append(headers, piping_util.KeyValue{Key: "Content-Type", Value: pmuxMimeType})
}

func Server(httpClient *http.Client, headers []piping_util.KeyValue, baseUploadUrl string, baseDownloadUrl string, enableHb bool, encrypts bool, passphrase string, cipherType string) *server {
	server := &server{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
		enableHb:        enableHb,
		encrypts:        encrypts,
		passphrase:      passphrase,
		cipherType:      cipherType,
	}
	go server.sendVersionAndConfigLoop()
	return server
}

type getSubPathStatusError struct {
	statusCode int
}

func (e *getSubPathStatusError) Error() string {
	return fmt.Sprintf("not status 200, found: %d", e.statusCode)
}

func (s *server) sendVersionAndConfigLoop() {
	b := backoff.NewExponentialBackoff()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
		defer cancel()
		// NOTE: In the future, config scheme can change more efficient format than JSON
		configJsonBytes, err := json.Marshal(serverConfigJson{Hb: s.enableHb})
		if err != nil {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
		postRes, err := piping_util.PipingSendWithContext(ctx, s.httpClient, headersWithPmux(s.headers), s.baseUploadUrl, bytes.NewReader(append(pmuxVersionBytes[:], configJsonBytes...)))
		if postRes.StatusCode != 200 {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
		// If timeout
		if util.IsTimeoutErr(err) {
			// reset backoff
			b.Reset()
			// No backoff
			continue
		}
		if err != nil {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
		_, err = io.Copy(ioutil.Discard, postRes.Body)
		if err != nil {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
	}
}

func (s *server) getSubPath() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	getRes, err := piping_util.PipingGetWithContext(ctx, s.httpClient, s.headers, s.baseDownloadUrl)
	if err != nil {
		return "", err
	}
	if getRes.StatusCode != 200 {
		return "", &getSubPathStatusError{statusCode: getRes.StatusCode}
	}
	resBytes, err := ioutil.ReadAll(getRes.Body)
	if err != nil {
		return "", err
	}
	var sync syncJson
	err = json.Unmarshal(resBytes, &sync)
	if err != nil {
		return "", err
	}
	return sync.SubPath, nil
}

func (s *server) Accept() (io.ReadWriteCloser, error) {
	b := backoff.NewExponentialBackoff()
	var subPath string
	for {
		var err error
		subPath, err = s.getSubPath()
		if err == nil {
			break
		}
		// If timeout
		if util.IsTimeoutErr(err) {
			// reset backoff
			b.Reset()
			// No backoff
			continue
		}
		// backoff
		time.Sleep(b.NextDuration())
	}
	uploadUrl, err := util.UrlJoin(s.baseUploadUrl, subPath)
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, subPath)
	if err != nil {
		return nil, err
	}
	var duplex io.ReadWriteCloser
	duplex, err = early_piping_duplex.DuplexConnect(s.httpClient, s.headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	if s.enableHb {
		duplex = hb_duplex.Duplex(duplex)
	}
	if s.encrypts {
		switch s.cipherType {
		case piping_util.CipherTypeAesCtr:
			// Encrypt with AES-CTR
			duplex, err = crypto_duplex.EncryptDuplexWithAesCtr(duplex, duplex, []byte(s.passphrase))
		case piping_util.CipherTypeOpenpgp:
			duplex, err = openpgp_duplex.SymmetricallyEncryptDuplexWithOpenPGP(duplex, duplex, []byte(s.passphrase))
		default:
			return nil, errors.Errorf("unexpected cipher type: %s", s.cipherType)
		}
	}
	return duplex, err
}

func Client(httpClient *http.Client, headers []piping_util.KeyValue, baseUploadUrl string, baseDownloadUrl string, enableHb bool, encrypts bool, passphrase string, cipherType string) (*client, error) {
	client := &client{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
		enableHb:        enableHb,
		encrypts:        encrypts,
		passphrase:      passphrase,
		cipherType:      cipherType,
	}
	return client, client.checkServerVersionAndConfig()
}

func (c *client) checkServerVersionAndConfig() error {
	b := backoff.NewExponentialBackoff()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
		defer cancel()
		postRes, err := piping_util.PipingGetWithContext(ctx, c.httpClient, c.headers, c.baseDownloadUrl)
		// If timeout
		if util.IsTimeoutErr(err) {
			// reset backoff
			b.Reset()
			// No backoff
			continue
		}
		if err != nil {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
		if postRes.Header.Get("Content-Type") != pmuxMimeType {
			return NonPmuxMimeTypeError
		}
		versionBytes := make([]byte, 4)
		_, err = io.ReadFull(postRes.Body, versionBytes)
		if err != nil {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
		serverVersion := binary.BigEndian.Uint32(versionBytes)
		if serverVersion != pmuxVersion {
			return IncompatiblePmuxVersion
		}
		serverConfigJsonBytes, err := io.ReadAll(postRes.Body)
		if err != nil {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
		var serverConfig serverConfigJson
		if json.Unmarshal(serverConfigJsonBytes, &serverConfig) != nil {
			return IncompatibleServerConfigError
		}
		if serverConfig.Hb != c.enableHb {
			return DifferentHbSettingError
		}
		return nil
	}
}

func (c *client) sendSubPath() (string, error) {
	subPath, err := util.RandomHexString()
	if err != nil {
		return "", err
	}
	sync := syncJson{SubPath: subPath}
	jsonBytes, err := json.Marshal(sync)
	if err != nil {
		return "", err
	}
	res, err := piping_util.PipingSend(c.httpClient, c.headers, c.baseUploadUrl, bytes.NewReader(jsonBytes))
	if err != nil {
		return "", err
	}
	if res.StatusCode != 200 {
		return "", errors.Errorf("not status 200, found: %d", res.StatusCode)
	}
	_, err = io.Copy(ioutil.Discard, res.Body)
	return subPath, err
}

func (c *client) Open() (io.ReadWriteCloser, error) {
	b := backoff.NewExponentialBackoff()
	var subPath string
	for {
		var err error
		subPath, err = c.sendSubPath()
		if err == nil {
			break
		}
		// If timeout
		if util.IsTimeoutErr(err) {
			b.Reset()
			continue
		}
		fmt.Fprintln(os.Stderr, "get sync error", err)
		time.Sleep(b.NextDuration())
	}
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, subPath)
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.UrlJoin(c.baseDownloadUrl, subPath)
	if err != nil {
		return nil, err
	}
	var duplex io.ReadWriteCloser
	duplex, err = early_piping_duplex.DuplexConnect(c.httpClient, c.headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	if c.enableHb {
		duplex = hb_duplex.Duplex(duplex)
	}
	if c.encrypts {
		switch c.cipherType {
		case piping_util.CipherTypeAesCtr:
			// Encrypt with AES-CTR
			duplex, err = crypto_duplex.EncryptDuplexWithAesCtr(duplex, duplex, []byte(c.passphrase))
		case piping_util.CipherTypeOpenpgp:
			duplex, err = openpgp_duplex.SymmetricallyEncryptDuplexWithOpenPGP(duplex, duplex, []byte(c.passphrase))
		default:
			return nil, errors.Errorf("unexpected cipher type: %s", c.cipherType)
		}
	}
	return duplex, err
}
