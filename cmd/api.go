/*
Copyright 2020 Mirantis, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
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

// APICommand creates new command for running the mke join API
func APICommand() *cli.Command {
	return &cli.Command{
		Name:   "api",
		Usage:  "Run the controller api",
		Action: startAPI,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "config",
				Value: "mke.yaml",
			},
		},
	}
}

var kubeClient k8s.Interface

func startAPI(ctx *cli.Context) error {
	clusterConfig, err := configFromCmdFlag(ctx)
	if err != nil {
		return err
	}

	kubeClient, err = kubernetes.Client(constant.AdminKubeconfigConfigPath)
	if err != nil {
		return err
	}
	prefix := "/v1beta1"
	router := mux.NewRouter()
	router.Use(authMiddleware)

	if clusterConfig.Spec.Storage.Type == v1beta1.EtcdStorageType {
		// Only mount the etcd handler if we're running on etcd storage
		// by default the mux will return 404 back which the caller should handle
		router.Path(prefix + "/etcd/members").Methods("POST").Handler(etcdHandler())
	}

	if clusterConfig.Spec.Storage.IsJoinable() {
		router.Path(prefix + "/ca").Methods("GET").Handler(caHandler())
	}

	srv := &http.Server{
		Handler:      router,
		Addr:         ":9443",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServeTLS(
		filepath.Join(constant.CertRootDir, "mke-api.crt"),
		filepath.Join(constant.CertRootDir, "mke-api.key"),
	))

	return nil
}

func etcdHandler() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
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

		memberList, err := etcdClient.AddMember(ctx, etcdReq.Node, etcdReq.PeerAddress)
		if err != nil {
			sendError(err, resp)
			return
		}

		etcdResp := v1beta1.EtcdResponse{
			InitialCluster: memberList,
		}

		etcdCaCertPath, etcdCaCertKey := filepath.Join(constant.EtcdCertDir, "ca.crt"), filepath.Join(constant.EtcdCertDir, "ca.key")
		etcdCACert, err := ioutil.ReadFile(etcdCaCertPath)
		if err != nil {
			sendError(err, resp)
			return
		}
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
		if err := json.NewEncoder(resp).Encode(etcdResp); err != nil {
			sendError(err, resp)
			return
		}
	})
}

func caHandler() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {

		caResp := v1beta1.CaResponse{}
		key, err := ioutil.ReadFile(path.Join(constant.CertRootDir, "ca.key"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.Key = key
		crt, err := ioutil.ReadFile(path.Join(constant.CertRootDir, "ca.crt"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.Cert = crt

		saKey, err := ioutil.ReadFile(path.Join(constant.CertRootDir, "sa.key"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.SAKey = saKey

		saPub, err := ioutil.ReadFile(path.Join(constant.CertRootDir, "sa.pub"))
		if err != nil {
			sendError(err, resp)
			return
		}
		caResp.SAPub = saPub

		resp.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(resp).Encode(caResp); err != nil {
			sendError(err, resp)
			return
		}
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
	if _, err := resp.Write([]byte(err.Error())); err != nil {
		sendError(err, resp)
		return
	}
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
	secret, err := kubeClient.CoreV1().Secrets("kube-system").Get(context.TODO(), secretName, v1.GetOptions{})
	if err != nil {
		logrus.Errorf("failed to get bootstrap token: %s", err.Error())
		return false
	}

	if string(secret.Data["token-secret"]) != parts[1] {
		return false
	}

	if string(secret.Data["usage-controller-join"]) != "true" {
		return false
	}
	return true
}
