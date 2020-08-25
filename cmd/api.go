package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	"github.com/Mirantis/mke/pkg/kubernetes"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/Mirantis/mke/pkg/etcd"
)

func ApiCommand() *cli.Command {
	return &cli.Command{
		Name:   "api",
		Usage:  "Run the controller api",
		Action: startApi,
		Flags:  []cli.Flag{},
	}
}

var kubeClient *k8s.Clientset

func startApi(ctx *cli.Context) error {
	var err error
	kubeClient, err = kubernetes.Client(filepath.Join(constant.CertRoot, "admin.conf"))
	if err != nil {
		return err
	}
	prefix := "/v1beta1"
	router := mux.NewRouter()
	router.Use(authMiddleware)

	router.Path(prefix + "/etcd").Methods("POST").Handler(etcdHandler())
	router.Path(prefix + "/ca").Methods("GET").Handler(caHandler())

	srv := &http.Server{
		Handler:      router,
		Addr:         ":9443",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServeTLS(
		filepath.Join(constant.CertRoot, "mke-api.crt"),
		filepath.Join(constant.CertRoot, "mke-api.key"),
	))

	return nil
}

func etcdHandler() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		logrus.Warn("********* etcd handler *********")
		var etcdReq v1beta1.EtcdRequest
		err := json.NewDecoder(req.Body).Decode(&etcdReq)
		if err != nil {
			sendError(err, resp)
			return
		}
		logrus.Infof("etcd API, adding new member: %s", etcdReq.PeerAddress)
		err = etcdReq.Validate()
		if err != nil {
			sendError(err, resp)
			return
		}

		etcdClient, err := etcd.NewClient()
		if err != nil {
			sendError(err, resp)
			return
		}

		memberList, err := etcdClient.AddMember(etcdReq.Node, etcdReq.PeerAddress)
		if err != nil {
			sendError(err, resp)
			return
		}

		etcdResp := v1beta1.EtcdResponse{
			InitialCluster: memberList,
		}

		etcdCaCertPath, etcdCaCertKey := filepath.Join(constant.CertRoot, "etcd", "ca.crt"), filepath.Join(constant.CertRoot, "etcd", "ca.key")
		etcdCACert, err := ioutil.ReadFile(etcdCaCertPath)
		etcdCAKey, err := ioutil.ReadFile(etcdCaCertKey)

		if err != nil {
			sendError(err, resp)
			return
		}

		etcdResp.CA = v1beta1.CaResponse{
			Key:  etcdCAKey,
			Cert: etcdCACert,
		}
		resp.Header().Set("content-type", "application/json")
		json.NewEncoder(resp).Encode(etcdResp)
	})
}

func caHandler() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {

		caResp := v1beta1.CaResponse{}
		key, err := ioutil.ReadFile(path.Join(constant.CertRoot, "ca.key"))
		if err != nil {
			sendError(err, resp)
		}
		caResp.Key = key
		crt, err := ioutil.ReadFile(path.Join(constant.CertRoot, "ca.crt"))
		if err != nil {
			sendError(err, resp)
		}
		caResp.Cert = crt

		resp.Header().Set("content-type", "application/json")
		json.NewEncoder(resp).Encode(caResp)
	})
}

func sendError(err error, resp http.ResponseWriter, status ...int) {
	code := http.StatusInternalServerError
	if len(status) == 1 {
		code = status[0]
	}

	logrus.Error(err)
	resp.Header().Set("Content-Type", "text/plain")
	resp.WriteHeader(code)
	resp.Write([]byte(err.Error()))
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			sendError(fmt.Errorf("Go away"), w, http.StatusUnauthorized)
			return
		}

		parts := strings.Split(auth, "Bearer ")
		if len(parts) == 2 {
			token := parts[1]
			if !isValidToken(token) {
				sendError(fmt.Errorf("Go away"), w, http.StatusUnauthorized)
				return
			}
		} else {
			sendError(fmt.Errorf("Go away"), w, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

/** The token is in form of xyz.foobar where:
- xyz: the token "ID" in kube api
- foobar: the token itself
We need to validate:
- that we find a secret with the ID
- that the token matches whats inside the secret
*/
func isValidToken(token string) bool {
	parts := strings.Split(token, ".")
	logrus.Debugf("token parts: %v", parts)
	if len(parts) != 2 {
		return false
	}

	secretName := fmt.Sprintf("bootstrap-token-%s", parts[0])
	secret, err := kubeClient.CoreV1().Secrets("kube-system").Get(secretName, v1.GetOptions{})
	if err != nil {
		logrus.Errorf("failed to get bootstrap token: %s", err.Error())
		return false
	}

	if string(secret.Data["token-secret"]) != parts[1] {
		return false
	}
	return true
}
