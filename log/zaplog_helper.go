package log

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/natefinch/lumberjack"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	ENV_LOG_FILE          = "env_log_file"
	ENV_LOG_FILE_WITH_PID = "env_log_file_with_pid"
)

var (
	Log         *zap.Logger
	Logf        *zap.SugaredLogger
	atomicLevel zap.AtomicLevel
)

type LoggerOption struct {
	FilePath   string //日志文件路径
	Level      string //日志级别: debug,info,warn,error,panic,fatal
	MaxSize    int32  //每个日志文件保存的最大尺寸 单位：M
	MaxBackups int32  //日志文件最多保存多少个备份
	MaxAge     int32  //文件最多保存多少天
	Compress   bool   //是否压缩
	Console    bool   //是否打印到屏幕上，缺少为不打印false
}

func NewLogger(opt *LoggerOption) *zap.Logger {
	var level zapcore.Level
	level.Set(strings.ToLower(opt.Level))
	return buildLogger(opt.FilePath, level, int(opt.MaxSize), int(opt.MaxBackups), int(opt.MaxAge), opt.Compress, opt.Console)
}

//   NewLogger 获取日志
//   filePath 日志文件路径
//   level 日志级别
//   maxSize 每个日志文件保存的最大尺寸 单位：M
//   maxBackups 日志文件最多保存多少个备份
//   maxAge 文件最多保存多少天
//   compress 是否压缩
//   serviceName 服务名
func buildLogger(filePath string, level zapcore.Level, maxSize int, maxBackups int, maxAge int, compress bool, console bool) *zap.Logger {
	core := newCore(filePath, level, maxSize, maxBackups, maxAge, compress, console)
	return zap.New(core, zap.AddCaller(), zap.Development())
}

//  newCore 构造日志模块
func newCore(filePath string, level zapcore.Level, maxSize int, maxBackups int, maxAge int, compress bool, console bool) zapcore.Core {
	//日志文件路径配置2
	hook := lumberjack.Logger{
		Filename:   filePath,   // 日志文件路径
		MaxSize:    maxSize,    // 每个日志文件保存的最大尺寸 单位：M
		MaxBackups: maxBackups, // 日志文件最多保存多少个备份
		MaxAge:     maxAge,     // 文件最多保存多少天
		Compress:   compress,   // 是否压缩
	}
	// 设置日志级别
	atomicLevel = zap.NewAtomicLevel()
	atomicLevel.SetLevel(level)

	//公用编码器
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,  // 小写编码器
		EncodeTime:     TcLoggerTimeEncoder,            //zapcore.ISO8601TimeEncoder,     // ISO8601 UTC 时间格式
		EncodeDuration: zapcore.SecondsDurationEncoder, //
		EncodeCaller:   zapcore.ShortCallerEncoder,     //zapcore.FullCallerEncoder
	}

	if console {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)), // 只打印到控制台
			atomicLevel, // 日志级别
		)
	} else {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.NewMultiWriteSyncer(zapcore.AddSync(&hook)), // 打印到控制台和文件
			atomicLevel, // 日志级别
		)
	}
}

func SetLogLevel(strLevel string) {
	var level zapcore.Level
	level.Set(strings.ToLower(strLevel))

	if Log != nil {
		atomicLevel.SetLevel(level)
	}
}

// TcLoggerTimeEncoder serializes a time.Time to an ISO8601-formatted string
// with millisecond precision.
func TcLoggerTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
}

const (
	defaultLogConfig string = `
		{
			"FilePath"  : "app.log",
			"Level"     : "info",
			"MaxSize"   : 500,
			"MaxBackups": 7,
			"MaxAge"    : 7,
			"Compress"  : true,
			"Console"   : true,
			"LevelValue": "debug,info,warn,error,panic,fatal"
		}
		`
)

func InitLogger(logConfigFile string) (*zap.Logger, error) {
	opt := &LoggerOption{}

	_, err := os.Stat(logConfigFile) //Stat获取文件属性
	if err != nil {
		json.Unmarshal([]byte(defaultLogConfig), opt)
	} else {
		lg := viper.New()
		lg.SetConfigFile(logConfigFile)
		lg.SetConfigType("json")
		if err := lg.ReadInConfig(); err != nil {
			fmt.Println("read log.json file fail!", err)
			return nil, err
		}
		if err := lg.Unmarshal(opt); err != nil {
			fmt.Println("log.json unmarshal fail!", err)
			return nil, err
		}
	}

	lfn := os.Getenv(ENV_LOG_FILE)
	if len(lfn) > 0 {
		opt.FilePath = lfn
	}

	if len(os.Getenv(ENV_LOG_FILE_WITH_PID)) > 0 {
		opt.FilePath = strings.Replace(opt.FilePath, ".log", fmt.Sprintf("-%d-%d.log", os.Getpid(), os.Getppid()), -1)
	}

	Log = NewLogger(opt)
	Logf = Log.Sugar()
	Log.Info("log init ok.", zap.String("LogLevel", opt.Level), zap.String("FilePath", opt.FilePath))

	return Log, nil
}
