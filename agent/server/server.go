package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/1Panel-dev/1Panel/agent/app/repo"
	"github.com/1Panel-dev/1Panel/agent/cron"
	"github.com/1Panel-dev/1Panel/agent/global"
	"github.com/1Panel-dev/1Panel/agent/i18n"
	"github.com/1Panel-dev/1Panel/agent/init/app"
	"github.com/1Panel-dev/1Panel/agent/init/business"
	"github.com/1Panel-dev/1Panel/agent/init/cache"
	"github.com/1Panel-dev/1Panel/agent/init/db"
	"github.com/1Panel-dev/1Panel/agent/init/dir"
	"github.com/1Panel-dev/1Panel/agent/init/firewall"
	"github.com/1Panel-dev/1Panel/agent/init/hook"
	"github.com/1Panel-dev/1Panel/agent/init/lang"
	"github.com/1Panel-dev/1Panel/agent/init/log"
	"github.com/1Panel-dev/1Panel/agent/init/migration"
	"github.com/1Panel-dev/1Panel/agent/init/router"
	"github.com/1Panel-dev/1Panel/agent/init/validator"
	"github.com/1Panel-dev/1Panel/agent/init/viper"
	"github.com/1Panel-dev/1Panel/agent/utils/encrypt"
	"github.com/1Panel-dev/1Panel/agent/utils/platform"
	"github.com/1Panel-dev/1Panel/agent/utils/re"
)

func Start() {
	re.Init()
	viper.Init()
	dir.Init()
	log.Init()
	global.LOG.Info("agent startup: logger initialized")
	db.Init()
	global.LOG.Info("agent startup: database initialized")
	migration.Init()
	global.LOG.Info("agent startup: migration initialized")
	i18n.Init()
	global.LOG.Info("agent startup: i18n initialized")
	cache.Init()
	global.LOG.Info("agent startup: cache initialized")
	app.Init()
	global.LOG.Info("agent startup: app initialized")
	lang.Init()
	global.LOG.Info("agent startup: language initialized")
	validator.Init()
	global.LOG.Info("agent startup: validator initialized")
	cron.Run()
	global.LOG.Info("agent startup: cron initialized")
	hook.Init()
	global.LOG.Info("agent startup: hook initialized")
	go firewall.Init()
	global.LOG.Info("agent startup: firewall init scheduled")
	InitOthers()
	global.LOG.Info("agent startup: edition initialized")

	rootRouter := router.Routers()
	global.LOG.Info("agent startup: router initialized")

	server := &http.Server{
		Handler: rootRouter,
	}

	if global.CONF.Base.Mode != "stable" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	if global.IsMaster {
		if platform.UseLocalAgentSocket() {
			sockPath := platform.LocalAgentSockPath
			global.LOG.Infof("agent startup: master mode, preparing unix socket %s", sockPath)
			if err := prepareMasterSocketDir(filepath.Dir(sockPath)); err != nil {
				panic(err)
			}
			_ = os.Remove(sockPath)
			listener, err := net.Listen("unix", sockPath)
			if err != nil {
				panic(err)
			}
			if err := secureMasterSocket(sockPath); err != nil {
				_ = listener.Close()
				panic(err)
			}
			global.LOG.Infof("agent startup: listening on unix socket %s", sockPath)
			business.Init()
			global.LOG.Info("agent startup: business initialized")
			_ = server.Serve(listener)
			return
		}

		server.Addr = platform.LocalAgentHTTPAddr
		business.Init()
		global.LOG.Infof("listen at http://%s", server.Addr)
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
		return
	} else {
		server.Addr = fmt.Sprintf("0.0.0.0:%s", global.CONF.Base.Port)
		global.LOG.Infof("agent startup: node mode, preparing https listener %s", server.Addr)
		settingRepo := repo.NewISettingRepo()
		certItem, err := settingRepo.Get(settingRepo.WithByKey("ServerCrt"))
		if err != nil {
			panic(err)
		}
		cert, _ := encrypt.StringDecrypt(certItem.Value)
		keyItem, err := settingRepo.Get(settingRepo.WithByKey("ServerKey"))
		if err != nil {
			panic(err)
		}
		key, _ := encrypt.StringDecrypt(keyItem.Value)
		tlsCert, err := tls.X509KeyPair([]byte(cert), []byte(key))
		if err != nil {
			fmt.Printf("failed to load X.509 key pair: %s\n", err)
			return
		}

		server.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}
		caItem, _ := settingRepo.GetValueByKey("RootCrt")
		if len(caItem) != 0 {
			caCertPool := x509.NewCertPool()
			rootCrt, _ := encrypt.StringDecrypt(caItem)
			caCertPool.AppendCertsFromPEM([]byte(rootCrt))
			server.TLSConfig.ClientCAs = caCertPool
		}
		business.Init()
		global.LOG.Info("agent startup: business initialized")
		global.LOG.Infof("listen at https://0.0.0.0:%s", global.CONF.Base.Port)
		if err := server.ListenAndServeTLS("", ""); err != nil {
			panic(err)
		}
	}
}
