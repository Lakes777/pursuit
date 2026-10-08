package candidaturas

// Etapa é onde uma candidatura está. Os valores são os mesmos da constraint no banco.
type Etapa string

const (
	Interesse  Etapa = "interesse"
	Enviada    Etapa = "enviada"
	Triagem    Etapa = "triagem"
	Entrevista Etapa = "entrevista"
	Tecnica    Etapa = "tecnica"
	Proposta   Etapa = "proposta"
	Contratado Etapa = "contratado"
	Recusada   Etapa = "recusada"
	Desisti    Etapa = "desisti"
)

// InfoEtapa é uma etapa com o nome para mostrar na tela.
type InfoEtapa struct {
	Etapa Etapa  `json:"etapa"`
	Nome  string `json:"nome"`
	// Final: a candidatura terminou (contratado, recusada ou desisti)
	Final bool `json:"final"`
}

// Etapas na ordem do processo seletivo; as três finais por último.
var Etapas = []InfoEtapa{
	{Interesse, "Interesse", false},
	{Enviada, "Candidatura enviada", false},
	{Triagem, "Triagem", false},
	{Entrevista, "Entrevista", false},
	{Tecnica, "Etapa técnica", false},
	{Proposta, "Proposta", false},
	{Contratado, "Contratado", true},
	{Recusada, "Recusada", true},
	{Desisti, "Desisti", true},
}

// Valida diz se a etapa existe.
func (e Etapa) Valida() bool {
	for _, info := range Etapas {
		if info.Etapa == e {
			return true
		}
	}
	return false
}
