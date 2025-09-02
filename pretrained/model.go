package pretrained

import (
	"fmt"
	"log"

	"github.com/season-studio/tokenizer"
	"github.com/season-studio/tokenizer/model"
	"github.com/season-studio/tokenizer/model/bpe"
	"github.com/season-studio/tokenizer/model/unigram"
	"github.com/season-studio/tokenizer/model/wordlevel"
	"github.com/season-studio/tokenizer/model/wordpiece"
	"github.com/season-studio/tokenizer/util"
)

// This file provides functions to create tokenizer.Model from input data.

func CreateModel(config *tokenizer.Config) (tokenizer.Model, error) {
	if config == nil {
		return nil, nil
	}

	params := util.NewParams(config.Model)

	var typ string
	if params.Has("type") {
		typ = params.Get("type").(string)
	} else {
		// Guessing from `decoder.type`
		dparams := util.NewParams(config.Decoder)
		if dparams.Has("type") {
			dtyp := dparams.Get("type").(string)
			switch dtyp {
			case "ByteLevel":
				typ = "BPE"
			case "WordPiece":
				typ = "WordPiece"
			case "WordLevel":
				typ = "WordLevel"
			case "Unigram":
				typ = "Unigram"
			default: // default to "BPE"
			}
		}
		if typ == "" {
			log.Printf("INFO: there is no field 'type' in model json data, a default 'BPE' model will be trying to create...\n")
			// typ = "WordPiece" // Default to `WordPiece` model as in BERT "tokenizer.json", there's not field "type"
			typ = "BPE" // Default to `WordPiece` model as in BERT "tokenizer.json", there's not field "type"
		}
	}

	switch typ {
	case "BPE":
		return createBPE(params)
	case "WordPiece":
		return createWordPiece(params)
	case "WordLevel":
		return createWordLevel(params)
	case "Unigram":
		return createUnigram(params)

	default:
		err := fmt.Errorf("Could not construct tokenizer.Model from input data: %#v\n", config)
		return nil, err
	}
}

// BPE json format:
// ----------------
// "type": "BPE",
// "dropout": null,
// "unk_token": null,
// "continuing_subword_prefix": null,
// "end_of_word_suffix": null,
// "fuse_unk": false,
// "byte_fallback": false,
// "vocab": {}
// "merges": []

func createBPE(params *util.Params) (tokenizer.Model, error) {
	var dropout *float32
	if params.Has("dropout") {
		val := float32(params.Get("dropout").(float64))
		dropout = &val
	}

	var unkToken *string
	if params.Has("unk_token") {
		v := params.Get("unk_token").(string)
		unkToken = &v
	}
	var continuingSubwordPrefix *string
	if params.Has("continuing_subword_prefix") {
		v := params.Get("continuing_subword_prefix").(string)
		continuingSubwordPrefix = &v
	}

	var endOfWordSuffix *string
	if params.Has("end_of_word_suffix") {
		v := params.Get("end_of_word_suffix").(string)
		endOfWordSuffix = &v
	}
	// fuseUnk := params.Get("use_unk").(bool)
	// byteFallback := params.Get("byte_fallback").(bool)

	vocab := castVocab(params.Get("vocab").(map[string]interface{}))
	merges, err := castMerge(params.Get("merges").([]interface{}))
	if err != nil {
		return nil, err
	}

	return bpe.New(vocab, merges, dropout, unkToken, continuingSubwordPrefix, endOfWordSuffix)
}

// WordPiece json format:
// ----------------------
// "unk_token": "[UNK]"
// "continuing_subword_prefix":"##"
// "max_input_chars_per_word":100
// "vocab": {}
// "decoder":{"type":"WordPiece","prefix":"##","cleanup":true},

func createWordPiece(params *util.Params) (tokenizer.Model, error) {
	opts := util.NewParams(nil)
	if params.Has("unk_token") {
		v := params.Get("unk_token").(string)
		opts.Set("unk_token", v)
	}
	if params.Has("continuing_subword_prefix") {
		v := params.Get("continuing_subword_prefix").(string)
		opts.Get("continuing_subword_prefix", v)
	}

	if params.Has("max_input_chars_per_word") {
		v := int(params.Get("max_input_chars_per_word").(float64))
		opts.Set("max_input_chars_per_word", v)
	}

	vocab := castVocab(params.Get("vocab").(map[string]interface{}))

	return wordpiece.New(vocab, opts)
}

func createWordLevel(params *util.Params) (tokenizer.Model, error) {
	var unkToken string
	if params.Has("unk_token") {
		v := params.Get("unk_token").(string)
		unkToken = v
	}

	vocab := castVocab(params.Get("vocab").(map[string]interface{}))

	return wordlevel.New(vocab, unkToken)
}

func createUnigram(params *util.Params) (tokenizer.Model, error) {
	// Extract parameters from the JSON configuration
	var unkID *int
	if params.Has("unk_id") {
		id := int(params.Get("unk_id").(float64))
		unkID = &id
	}

	bytesFallback := false
	if params.Has("byte_fallback") {
		bytesFallback = params.Get("byte_fallback").(bool)
	}

	fuseUnk := true
	if params.Has("fuse_unk") {
		fuseUnk = params.Get("fuse_unk").(bool)
	}

	// Extract the vocabulary
	var vocab []unigram.TokenScore
	if params.Has("vocab") {
		vocabData := params.Get("vocab").([]interface{})
		vocab = make([]unigram.TokenScore, len(vocabData))

		for i, entry := range vocabData {
			pair := entry.([]interface{})
			if len(pair) != 2 {
				return nil, fmt.Errorf("invalid vocabulary entry format: %v", pair)
			}

			token := pair[0].(string)
			score := pair[1].(float64)

			vocab[i] = unigram.TokenScore{
				Token: token,
				Score: score,
			}
		}
	} else {
		return nil, fmt.Errorf("unigram model requires a vocabulary")
	}

	// Create options for the Unigram model
	opts := util.NewParams(nil)
	if unkID != nil {
		opts.Set("unk_id", *unkID)
	}
	opts.Set("byte_fallback", bytesFallback)
	opts.Set("fuse_unk", fuseUnk)

	// Create and return the Unigram model
	return unigram.New(vocab, opts)
}

func castVocab(input map[string]interface{}) model.Vocab {
	out := make(map[string]int)
	for k, v := range input {
		out[k] = int(v.(float64))
	}

	return out
}

func castMerge(input []interface{}) ([]string, error) {
	out := make([]string, len(input))
	for i, v := range input {
		switch vTyped := v.(type) {
		case []interface{}:
			if len(vTyped) != 2 {
				return nil, fmt.Errorf("invalid merge format: %#v should be of length 2", vTyped)
			}
			out[i] = vTyped[0].(string) + " " + vTyped[1].(string)
		case []string:
			if len(vTyped) != 2 {
				return nil, fmt.Errorf("invalid merge format: %#v should be of length 2", vTyped)
			}
			out[i] = vTyped[0] + " " + vTyped[1]
		case string:
			out[i] = vTyped
		}
	}

	return out, nil
}
