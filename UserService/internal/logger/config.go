package logger

type Config struct {
	Service      string
	Env          string
	Version      string
	Level        string
	Format       string
	Output       string
	FilePath     string
	ErrorPath    string
	MaxSizeMB    int
	MaxBackups   int
	MaxAgeDays   int
	CompressFile bool
}
