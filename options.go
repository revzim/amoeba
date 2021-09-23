package amoeba

import (
	"net/http"
	"time"

	"github.com/revzim/amoeba/auth"
	"github.com/revzim/amoeba/cluster"
	"github.com/revzim/amoeba/component"

	// "github.com/revzim/amoeba/drivers"
	"github.com/revzim/amoeba/internal/env"
	"github.com/revzim/amoeba/internal/log"
	"github.com/revzim/amoeba/internal/message"
	"github.com/revzim/amoeba/pipeline"
	"github.com/revzim/amoeba/serialize"
	"github.com/revzim/azdrivers"
	"google.golang.org/grpc"
)

type Option func(*cluster.Options)

func WithJWT(authJWT *auth.JWT) Option {
	return func(_ *cluster.Options) {
		env.JWT = authJWT
	}
}

func WithJWTOpts(signKey, algo string, genTokenFunc auth.JWTFunc) Option {
	return func(_ *cluster.Options) {
		env.JWT = auth.NewJWT(signKey, algo, genTokenFunc)
	}
}

func WithMongo(uri string) Option {
	return func(_ *cluster.Options) {
		mongoApp, err := azdrivers.NewMongoApp(uri)
		if err != nil {
			log.Fatal(err.Error())
		}
		env.MongoDriver = mongoApp
	}
}

func WithFirebase(path string) Option {
	return func(_ *cluster.Options) {
		fbApp, err := azdrivers.NewFirebaseApp(path)
		if err != nil {
			log.Fatal(err.Error())
		}
		env.FirebaseDriver = fbApp
	}
}

func WithPipeline(pipeline pipeline.Pipeline) Option {
	return func(opt *cluster.Options) {
		opt.Pipeline = pipeline
	}
}

// WithAdvertiseAddr sets the advertise address option, it will be the listen address in
// master node and an advertise address which cluster member to connect
func WithAdvertiseAddr(addr string, retryInterval ...time.Duration) Option {
	return func(opt *cluster.Options) {
		opt.AdvertiseAddr = addr
		if len(retryInterval) > 0 {
			opt.RetryInterval = retryInterval[0]
		}
	}
}

// WithMemberAddr sets the listen address which is used to establish connection between
// cluster members. Will select an available port automatically if no member address
// setting and panic if no available port
func WithClientAddr(addr string) Option {
	return func(opt *cluster.Options) {
		opt.ClientAddr = addr
	}
}

// WithMaster sets the option to indicate whether the current node is master node
func WithMaster() Option {
	return func(opt *cluster.Options) {
		opt.IsMaster = true
	}
}

// WithGrpcOptions sets the grpc dial options
func WithGrpcOptions(opts ...grpc.DialOption) Option {
	return func(_ *cluster.Options) {
		env.GrpcOptions = append(env.GrpcOptions, opts...)
	}
}

// WithComponents sets the Components
func WithComponents(components *component.Components) Option {
	return func(opt *cluster.Options) {
		opt.Components = components
	}
}

// WithHeartbeatInterval sets Heartbeat time interval
func WithHeartbeatInterval(d time.Duration) Option {
	return func(_ *cluster.Options) {
		env.Heartbeat = d
	}
}

// WithCheckOriginFunc sets the function that check `Origin` in http headers
func WithCheckOriginFunc(fn func(*http.Request) bool) Option {
	return func(opt *cluster.Options) {
		env.CheckOrigin = fn
	}
}

// WithDebugMode let 'amoeba' to run under Debug mode.
func WithDebugMode() Option {
	return func(_ *cluster.Options) {
		env.Debug = true
	}
}

// SetDictionary sets routes map
func WithDictionary(dict map[string]uint16) Option {
	return func(_ *cluster.Options) {
		message.SetDictionary(dict)
	}
}

func WithWSPath(path string) Option {
	return func(_ *cluster.Options) {
		env.WSPath = path
	}
}

// SetTimerPrecision sets the ticker precision, and time precision can not less
// than a Millisecond, and can not change after application running. The default
// precision is time.Second
func WithTimerPrecision(precision time.Duration) Option {
	if precision < time.Millisecond {
		panic("time precision can not less than a Millisecond")
	}
	return func(_ *cluster.Options) {
		env.TimerPrecision = precision
	}
}

// WithSerializer customizes application serializer, which automatically Marshal
// and UnMarshal handler payload
func WithSerializer(serializer serialize.Serializer) Option {
	return func(opt *cluster.Options) {
		env.Serializer = serializer
	}
}

// WithLabel sets the current node label in cluster
func WithLabel(label string) Option {
	return func(opt *cluster.Options) {
		opt.Label = label
	}
}

// WithIsWebsocket indicates whether current node WebSocket is enabled
func WithIsWebsocket(enableWs bool) Option {
	return func(opt *cluster.Options) {
		opt.IsWebsocket = enableWs
	}
}

// WithTSLConfig sets the `key` and `certificate` of TSL
func WithTSLConfig(certificate, key string) Option {
	return func(opt *cluster.Options) {
		opt.TSLCertificate = certificate
		opt.TSLKey = key
	}
}

// WithLogger overrides the default logger
func WithLogger(l log.Logger) Option {
	return func(opt *cluster.Options) {
		log.SetLogger(l)
	}
}

// WithHandshakeValidator sets the function that Verify `handshake` data
func WithHandshakeValidator(fn func([]byte) error) Option {
	return func(opt *cluster.Options) {
		env.HandshakeValidator = fn
	}
}
