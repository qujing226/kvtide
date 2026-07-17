package tokenizer

import (
	"path/filepath"

	gotokenizer "github.com/qujing226/gotokenizer"
	"github.com/qujing226/kvtide/internal/conf"
)

func newQwen3Tokenizer(modelConf conf.ModelConf) (tokenize, error) {
	t, err := gotokenizer.NewQwenTokenizer(gotokenizer.QwenTokenizerConfig{
		VocabPath:           filepath.Join(modelConf.ModelPath, "vocab.json"),
		MergesPath:          filepath.Join(modelConf.ModelPath, "merges.txt"),
		TokenizerConfigPath: filepath.Join(modelConf.ModelPath, "tokenizer_config.json"),
	})
	return t, err
}
