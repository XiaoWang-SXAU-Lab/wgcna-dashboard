package regutrace

import _ "embed"

// JASPAR 2024 CORE non-redundant PFM collections, grouped by JASPAR taxon.
// The full set is under 1 MB, so all taxa are bundled for offline use.
//
//go:embed assets/JASPAR2024_CORE_plants_non-redundant_pfms_jaspar.txt
var jasparPlants []byte

//go:embed assets/JASPAR2024_CORE_vertebrates_non-redundant_pfms_jaspar.txt
var jasparVertebrates []byte

//go:embed assets/JASPAR2024_CORE_insects_non-redundant_pfms_jaspar.txt
var jasparInsects []byte

//go:embed assets/JASPAR2024_CORE_nematodes_non-redundant_pfms_jaspar.txt
var jasparNematodes []byte

//go:embed assets/JASPAR2024_CORE_fungi_non-redundant_pfms_jaspar.txt
var jasparFungi []byte

//go:embed assets/JASPAR2024_CORE_urochordates_non-redundant_pfms_jaspar.txt
var jasparUrochordates []byte

//go:embed assets/JASPAR2024_CORE_diatoms_non-redundant_pfms_jaspar.txt
var jasparDiatoms []byte

// BuiltinLibrary is one bundled JASPAR taxon collection.
type BuiltinLibrary struct {
	Key   string
	Name  string
	Data  []byte
	Count int
}

// DefaultMotifLibrary is used when no library key is supplied by an older
// client.
const DefaultMotifLibrary = "plants"

// BuiltinLibraries is ordered for the frontend kingdom selector.
var BuiltinLibraries = []*BuiltinLibrary{
	{Key: "plants", Name: "JASPAR 2024 CORE plants (non-redundant)", Data: jasparPlants},
	{Key: "vertebrates", Name: "JASPAR 2024 CORE vertebrates (non-redundant)", Data: jasparVertebrates},
	{Key: "insects", Name: "JASPAR 2024 CORE insects (non-redundant)", Data: jasparInsects},
	{Key: "nematodes", Name: "JASPAR 2024 CORE nematodes (non-redundant)", Data: jasparNematodes},
	{Key: "fungi", Name: "JASPAR 2024 CORE fungi (non-redundant)", Data: jasparFungi},
	{Key: "urochordates", Name: "JASPAR 2024 CORE urochordates (non-redundant)", Data: jasparUrochordates},
	{Key: "diatoms", Name: "JASPAR 2024 CORE diatoms (non-redundant)", Data: jasparDiatoms},
}

func builtinLibrary(key string) (*BuiltinLibrary, bool) {
	for _, lib := range BuiltinLibraries {
		if lib.Key == key {
			return lib, true
		}
	}
	return nil, false
}
