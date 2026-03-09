package dataset

import "math/rand/v2"

// Bigrams are the 200 most common English bigrams ranked by frequency.
// Source: ranelpadon/ngram-type
var Bigrams = []string{
	"th", "he", "in", "er", "an", "re", "on", "at", "en", "nd",
	"ti", "es", "or", "te", "of", "ed", "is", "it", "al", "ar",
	"st", "to", "nt", "ng", "se", "ha", "as", "ou", "io", "le",
	"ve", "co", "me", "de", "hi", "ri", "ro", "ic", "ne", "ea",
	"ra", "ce", "li", "ch", "ll", "be", "ma", "si", "om", "ur",
	"ca", "el", "ta", "la", "ns", "di", "fo", "ho", "pe", "ec",
	"pr", "no", "ct", "us", "ac", "ot", "il", "tr", "ly", "nc",
	"et", "ut", "ss", "so", "rs", "un", "lo", "wa", "ge", "ie",
	"wh", "ee", "wi", "em", "ad", "ol", "rt", "po", "we", "na",
	"ul", "ni", "ts", "mo", "ow", "pa", "im", "mi", "ai", "sh",
	"ir", "su", "id", "os", "iv", "ia", "am", "fi", "ci", "vi",
	"pl", "ig", "tu", "ev", "ld", "ry", "mp", "fe", "bl", "ab",
	"gh", "ty", "op", "wo", "sa", "ay", "ex", "ke", "fr", "oo",
	"av", "ag", "if", "ap", "gr", "od", "bo", "sp", "rd", "do",
	"uc", "bu", "ei", "ov", "by", "rm", "ep", "tt", "oc", "fa",
	"ef", "cu", "rn", "sc", "gi", "da", "yo", "cr", "cl", "du",
	"ga", "qu", "ue", "ff", "ba", "ey", "ls", "va", "um", "pp",
	"ua", "up", "lu", "go", "ht", "ru", "ug", "ds", "lt", "pi",
	"rc", "rr", "eg", "au", "ck", "ew", "mu", "br", "bi", "pt",
	"ak", "pu", "ui", "rg", "ib", "tl", "ny", "ki", "rk", "ys",
}

// Trigrams are the 200 most common English trigrams ranked by frequency.
// Source: ranelpadon/ngram-type
var Trigrams = []string{
	"the", "and", "ing", "ion", "tio", "ent", "ati", "for", "her", "ter",
	"hat", "tha", "ere", "ate", "his", "con", "res", "ver", "all", "ons",
	"nce", "men", "ith", "ted", "ers", "pro", "thi", "wit", "are", "ess",
	"not", "ive", "was", "ect", "rea", "com", "eve", "per", "int", "est",
	"sta", "cti", "ica", "ist", "ear", "ain", "one", "our", "iti", "rat",
	"nte", "tin", "ine", "der", "ome", "man", "pre", "rom", "tra", "whi",
	"ave", "str", "act", "ill", "ure", "ide", "ove", "cal", "ble", "out",
	"sti", "tic", "oun", "enc", "ore", "ant", "ity", "fro", "art", "tur",
	"par", "red", "oth", "eri", "hic", "ies", "ste", "ght", "ich", "igh",
	"und", "you", "ort", "era", "wer", "nti", "oul", "nde", "ind", "tho",
	"hou", "nal", "but", "hav", "uld", "use", "han", "hin", "een", "ces",
	"cou", "lat", "tor", "ese", "age", "ame", "rin", "anc", "ten", "hen",
	"min", "eas", "can", "lit", "cha", "ous", "eat", "end", "ssi", "ial",
	"les", "ren", "tiv", "nts", "whe", "tat", "abl", "dis", "ran", "wor",
	"rou", "lin", "had", "sed", "ont", "ple", "ugh", "inc", "sio", "din",
	"ral", "ust", "tan", "nat", "ins", "ass", "pla", "ven", "ell", "she",
	"ose", "ite", "lly", "rec", "lan", "ard", "hey", "rie", "pos", "eme",
	"mor", "den", "oug", "tte", "ned", "rit", "ime", "sin", "ast", "any",
	"orm", "ndi", "ona", "spe", "ene", "hei", "ric", "ice", "ord", "omp",
	"nes", "sen", "tim", "tri", "ern", "tes", "por", "app", "lar", "ntr",
}

// GenerateNgramLessons builds a list of lessons from ngrams.
// It slices the pool to `scope`, shuffles, walks in `combination`-sized chunks,
// and repeats each chunk `repetition` times to form one lesson.
// Returns [][]string where each inner slice is one lesson's word list.
func GenerateNgramLessons(ngrams []string, scope, combination, repetition int) [][]string {
	if scope <= 0 || scope > len(ngrams) {
		scope = len(ngrams)
	}

	pool := make([]string, scope)
	copy(pool, ngrams[:scope])
	rand.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})

	var lessons [][]string
	for i := 0; i < len(pool); i += combination {
		end := min(i+combination, len(pool))
		chunk := pool[i:end]
		var lesson []string
		for r := 0; r < repetition; r++ {
			lesson = append(lesson, chunk...)
		}
		lessons = append(lessons, lesson)
	}

	return lessons
}
