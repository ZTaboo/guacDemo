package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"learn/guac"
	"net/http"
)

type GuacArgs struct {
	GuacadAddr    string `form:"guacad_addr"`    // 鳄梨酱网关地址
	AssetProtocol string `form:"asset_protocol"` // 客户端(vnc/rdp)地址
	AssetHost     string `form:"asset_host"`     // 客户端主机
	AssetPort     string `form:"asset_port"`     // 客户端端口
	AssetUser     string `form:"asset_user"`     // 客户端用户名
	AssetPassword string `form:"asset_password"` // 客户端密码
	ScreenWidth   int    `form:"screen_width"`   // 屏幕宽度
	ScreenHeight  int    `form:"screen_height"`  // 屏幕高度
	ScreenDpi     int    `form:"screen_dpi"`     // 屏幕分辨率
}

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})
	r.GET("/ws", initGuac)
	r.Run() // listen and serve on 0.0.0.0:8080
}

func initGuac(c *gin.Context) {
	websocketReadBufferSize := guac.MaxGuacMessage
	websocketWriteBufferSize := guac.MaxGuacMessage * 2
	upgrade := websocket.Upgrader{
		ReadBufferSize:  websocketReadBufferSize,
		WriteBufferSize: websocketWriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			//检查origin 限定websocket 被其他的域名访问
			return true
		},
	}
	protocol := c.Request.Header.Get("Sec-Websocket-Protocol")
	ws, err := upgrade.Upgrade(c.Writer, c.Request, http.Header{
		"Sec-Websocket-Protocol": {protocol},
	})
	if err != nil {
		logrus.WithError(err).Error("升级ws失败")
		return
	}
	defer ws.Close()
	var args = GuacArgs{
		GuacadAddr:    "localhost:4822",
		AssetProtocol: "ssh",
		AssetHost:     "192.168.152.129",
		AssetPort:     "22",
		AssetUser:     "zero",
		AssetPassword: "zero",
		ScreenWidth:   1024,
		ScreenHeight:  760,
		ScreenDpi:     100,
	}
	uid := ""
	tunnel, err := guac.NewGuacamoleTunnel(args.GuacadAddr, args.AssetProtocol, args.AssetHost, args.AssetPort, args.AssetUser, args.AssetPassword, uid, args.ScreenWidth, args.ScreenHeight, args.ScreenDpi)
	if err != nil {
		logrus.Errorln(err)
		return
	}
	defer tunnel.Close()
	ioCopy(ws, tunnel)
}

func ioCopy(ws *websocket.Conn, tunnl *guac.SimpleTunnel) {

	writer := tunnl.AcquireWriter()
	reader := tunnl.AcquireReader()
	//if pipeTunnel.OnDisconnectWs != nil {
	//	defer pipeTunnel.OnDisconnectWs(id, ws, c.Request, pipeTunnel.TunnelPipe)
	//}
	defer tunnl.ReleaseWriter()
	defer tunnl.ReleaseReader()

	//使用 errgroup 来处理(管理) goroutine for-loop, 防止 for-goroutine zombie
	eg, _ := errgroup.WithContext(context.Background())

	eg.Go(func() error {
		buf := bytes.NewBuffer(make([]byte, 0, guac.MaxGuacMessage*2))

		for {
			ins, err := reader.ReadSome()
			if err != nil {
				return err
			}

			if bytes.HasPrefix(ins, guac.InternalOpcodeIns) {
				// messages starting with the InternalDataOpcode are never sent to the websocket
				continue
			}

			if _, err = buf.Write(ins); err != nil {
				return err
			}

			// if the buffer has more data in it or we've reached the max buffer size, send the data and reset
			if !reader.Available() || buf.Len() >= guac.MaxGuacMessage {
				if err = ws.WriteMessage(1, buf.Bytes()); err != nil {
					if err == websocket.ErrCloseSent {
						return fmt.Errorf("websocket:%v", err)
					}
					logrus.Traceln("Failed sending message to ws", err)
					return err
				}
				buf.Reset()
			}
		}

	})
	eg.Go(func() error {
		for {
			_, data, err := ws.ReadMessage()
			if err != nil {
				logrus.Traceln("Error reading message from ws", err)
				return err
			}
			if bytes.HasPrefix(data, guac.InternalOpcodeIns) {
				// messages starting with the InternalDataOpcode are never sent to guacd
				continue
			}
			if _, err = writer.Write(data); err != nil {
				logrus.Traceln("Failed writing to guacd", err)
				return err
			}
		}

	})
	if err := eg.Wait(); err != nil {
		logrus.WithError(err).Error("session-err")
	}

}
