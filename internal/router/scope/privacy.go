package scope

// 공유/전파 정책을 위한 기본 규칙.
// - 기본: workspace/org 공유는 "옵트인"
// - user scope는 원칙적으로 외부로 export 불가(또는 익명화)
type Privacy struct {
	AllowExportUserScope   bool // 기본 false
	AllowImportToUserScope bool // 기본 false

	// workspace/org 공유 시 식별자 최소화
	RedactProviderKeys    bool // API key 같은 건 절대 포함 금지(원천적으로 저장도 금지)
	RedactExactModelNames bool // 필요 시 모델명을 버킷(large/medium/small)로 변환 가능
}

func DefaultPrivacy() Privacy {
	return Privacy{
		AllowExportUserScope:   false,
		AllowImportToUserScope: false,
		RedactProviderKeys:     true,
		RedactExactModelNames:  false,
	}
}
