package iban

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

const (
	CountryCode = "FR"
	BankCode    = "30004" // Code banque fictif Binary (5 chiffres)
	BranchCode  = "00812" // Code guichet fictif (5 chiffres)
)

// Generate génère un IBAN français (27 caractères) mathématiquement valide selon ISO 13616.
func Generate() (string, error) {
	// Génération d'un numéro de compte aléatoire sur 11 chiffres
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(11), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("génération numéro de compte aléatoire: %w", err)
	}
	accountNum := fmt.Sprintf("%011d", n)

	// Clé RIB sur 2 chiffres (calcul simplifié pour BBAN FR : 97 - ((89*Banque + 15*Guichet + 3*Compte) mod 97))
	ribKey := computeRIBKey(BankCode, BranchCode, accountNum)
	bban := BankCode + BranchCode + accountNum + ribKey

	// Calcul de la clé IBAN ISO 13616 : 98 - (numeric(BBAN + CountryCode + "00") mod 97)
	checkDigits := computeISO13616CheckDigits(CountryCode, bban)

	return CountryCode + checkDigits + bban, nil
}

// Validate vérifie si une chaîne respecte la structure et le contrôle Modulo 97-10 ISO 13616.
func Validate(rawIBAN string) bool {
	cleaned := strings.ReplaceAll(strings.ToUpper(strings.TrimSpace(rawIBAN)), " ", "")
	if len(cleaned) < 15 || len(cleaned) > 34 {
		return false
	}

	// Déplacer les 4 premiers caractères à la fin
	rearranged := cleaned[4:] + cleaned[:4]
	numericStr := toNumericString(rearranged)
	if numericStr == "" {
		return false
	}

	val, ok := new(big.Int).SetString(numericStr, 10)
	if !ok {
		return false
	}

	mod := new(big.Int).Mod(val, big.NewInt(97))
	return mod.Int64() == 1
}

func computeISO13616CheckDigits(country, bban string) string {
	numericStr := toNumericString(bban + country + "00")
	val, _ := new(big.Int).SetString(numericStr, 10)
	mod := new(big.Int).Mod(val, big.NewInt(97))
	check := 98 - mod.Int64()
	return fmt.Sprintf("%02d", check)
}

func computeRIBKey(bank, branch, account string) string {
	b, _ := new(big.Int).SetString(bank, 10)
	g, _ := new(big.Int).SetString(branch, 10)
	c, _ := new(big.Int).SetString(account, 10)

	// Formule officielle RIB : 97 - ((89*B + 15*G + 3*C) mod 97)
	term1 := new(big.Int).Mul(b, big.NewInt(89))
	term2 := new(big.Int).Mul(g, big.NewInt(15))
	term3 := new(big.Int).Mul(c, big.NewInt(3))

	sum := new(big.Int).Add(term1, term2)
	sum.Add(sum, term3)

	mod := new(big.Int).Mod(sum, big.NewInt(97))
	key := 97 - mod.Int64()
	return fmt.Sprintf("%02d", key)
}

func toNumericString(input string) string {
	var sb strings.Builder
	for _, r := range input {
		switch {
		case r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			// A=10, B=11 ... Z=35
			sb.WriteString(fmt.Sprintf("%d", r-'A'+10))
		default:
			return ""
		}
	}
	return sb.String()
}
