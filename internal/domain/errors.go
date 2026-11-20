package domain

import "errors"

var (
	ErrAccountNotFound     = errors.New("compte bancaire introuvable")
	ErrAccountFrozen       = errors.New("compte bancaire gelé par décision réglementaire")
	ErrAccountClosed       = errors.New("compte bancaire clôturé")
	ErrInsufficientFunds   = errors.New("solde disponible insuffisant")
	ErrCurrencyMismatch    = errors.New("les devises des comptes ne correspondent pas")
	ErrInvalidAmount       = errors.New("le montant doit être strictement positif")
	ErrSameAccountTransfer = errors.New("les comptes source et destination doivent être différents")

	ErrUnauthorizedAction   = errors.New("action non autorisée pour ce rôle")
	ErrSelfApprovalDenied   = errors.New("un collaborateur ne peut pas approuver sa propre opération")
	ErrInvalidTxState       = errors.New("statut de transaction invalide pour cette opération")
	ErrDuplicateIdempotency = errors.New("clé d'idempotence déjà traitée")

	ErrInvalidCredentials = errors.New("email ou mot de passe incorrect")
	ErrAccountSuspended   = errors.New("compte utilisateur suspendu ou bloqué")
	ErrInvalidSession     = errors.New("session expirée ou révoquée")

	ErrUnsupportedCurrency   = errors.New("devise non supportée (EUR, USD, GBP uniquement)")
	ErrNonZeroAccountBalance = errors.New("impossible de clôturer un compte dont le solde n'est pas nul")
)
