package manifestadapter

// ParsingOptions Options that are given to the parser in the api package
type ParsingOptions struct {
	ExtractedFilesFormats []string `mapstructure:"extracted-files-formats"`
	Hugo                  bool     `mapstructure:"hugo"`
}
