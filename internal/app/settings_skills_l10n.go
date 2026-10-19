package app

// Texts of the skills tab in the settings window.
const (
	xSkillsSettingsTab    textKey = "skillsSettingsTab"
	xSkillsTrash          textKey = "skillsTrash"
	xSkillsAll            textKey = "skillsAll"
	xSkillsSourceAll      textKey = "skillsSourceAll"
	xSkillsProviderAll    textKey = "skillsProviderAll"
	xSkillsFilter         textKey = "skillsFilter"
	xSkillsSearchMore     textKey = "skillsSearchMore"
	xSkillsFlagShared     textKey = "skillsFlagShared"
	xSkillsFlagLink       textKey = "skillsFlagLink"
	xSkillsFlagGitHub     textKey = "skillsFlagGitHub"
	xSkillsFlagIssues     textKey = "skillsFlagIssues"
	xSkillsFlagDeletable  textKey = "skillsFlagDeletable"
	xSkillsFlagSwitchable textKey = "skillsFlagSwitchable"
	xSkillsClearFilters   textKey = "skillsClearFilters"
	xSkillsSort           textKey = "skillsSort"
	xSkillsSortCLI        textKey = "skillsSortCLI"
	xSkillsSortName       textKey = "skillsSortName"
	xSkillsSortModified   textKey = "skillsSortModified"
	xSkillsShowing        textKey = "skillsShowing"
	xSkillsScannedAt      textKey = "skillsScannedAt"
	xSkillsRoots          textKey = "skillsRoots"
	xSkillsRootsHint      textKey = "skillsRootsHint"
	xSkillsRootMissing    textKey = "skillsRootMissing"
	xSkillsCount          textKey = "skillsCount"
	xSkillsShowIssues     textKey = "skillsShowIssues"
	xSkillsHideIssues     textKey = "skillsHideIssues"
	xSkillsIssuesHint     textKey = "skillsIssuesHint"
	xSkillsSharedCompact  textKey = "skillsSharedCompact"

	xSkillsBack                   textKey = "skillsBack"
	xSkillsOpenRepo               textKey = "skillsOpenRepo"
	xSkillsOpenOnline             textKey = "skillsOpenOnline"
	xSkillsOpenHomepage           textKey = "skillsOpenHomepage"
	xSkillsCopyPath               textKey = "skillsCopyPath"
	xSkillsCopied                 textKey = "skillsCopied"
	xSkillsTransfer               textKey = "skillsTransfer"
	xSkillsCopyTo                 textKey = "skillsCopyTo"
	xSkillsMoveTo                 textKey = "skillsMoveTo"
	xSkillsMove                   textKey = "skillsMove"
	xSkillsMoveTitle              textKey = "skillsMoveTitle"
	xSkillsMoveMessage            textKey = "skillsMoveMessage"
	xSkillsCopiedTo               textKey = "skillsCopiedTo"
	xSkillsMovedTo                textKey = "skillsMovedTo"
	xSkillsTransferFailed         textKey = "skillsTransferFailed"
	xSkillsTransferProviderError  textKey = "skillsTransferProviderError"
	xSkillsTransferConflict       textKey = "skillsTransferConflict"
	xSkillsTransferSource         textKey = "skillsTransferSource"
	xSkillsTransferOverlap        textKey = "skillsTransferOverlap"
	xSkillsTransferUnsafeLink     textKey = "skillsTransferUnsafeLink"
	xSkillsTransferNotMovable     textKey = "skillsTransferNotMovable"
	xSkillsTransferPartial        textKey = "skillsTransferPartial"
	xSkillsTransferPartialUnknown textKey = "skillsTransferPartialUnknown"
	xSkillsMovedTrash             textKey = "skillsMovedTrash"
	xSkillsMovedSystem            textKey = "skillsMovedSystem"
	xSkillsReason                 textKey = "skillsReason"
	xSkillsSwitch                 textKey = "skillsSwitch"
	xSkillsSecBasics              textKey = "skillsSecBasics"
	xSkillsDisplayName            textKey = "skillsDisplayName"
	xSkillsScope                  textKey = "skillsScope"
	xSkillsScopeUser              textKey = "skillsScopeUser"
	xSkillsRealPath               textKey = "skillsRealPath"

	xSkillsSecOrigin     textKey = "skillsSecOrigin"
	xSkillsOriginUnknown textKey = "skillsOriginUnknown"
	xSkillsInstaller     textKey = "skillsInstaller"
	xSkillsInstSkillsCLI textKey = "skillsInstSkillsCLI"
	xSkillsInstGit       textKey = "skillsInstGit"
	xSkillsInstMarket    textKey = "skillsInstMarket"
	xSkillsInstFront     textKey = "skillsInstFront"
	xSkillsRepository    textKey = "skillsRepository"
	xSkillsRepoPath      textKey = "skillsRepoPath"
	xSkillsBranch        textKey = "skillsBranch"
	xSkillsHomepage      textKey = "skillsHomepage"
	xSkillsVersion       textKey = "skillsVersion"
	xSkillsInstalledAt   textKey = "skillsInstalledAt"
	xSkillsUpdatedAt     textKey = "skillsUpdatedAt"

	xSkillsSecFiles      textKey = "skillsSecFiles"
	xSkillsFilesCount    textKey = "skillsFilesCount"
	xSkillsFilesPartial  textKey = "skillsFilesPartial"
	xSkillsFilesMore     textKey = "skillsFilesMore"
	xSkillsFilesLess     textKey = "skillsFilesLess"
	xSkillsNoFiles       textKey = "skillsNoFiles"
	xSkillsSkippedDir    textKey = "skillsSkippedDir"
	xSkillsInspectFailed textKey = "skillsInspectFailed"
	xSkillsFrontHint     textKey = "skillsFrontHint"
	xSkillsBodyHint      textKey = "skillsBodyHint"
	xSkillsBodyHidden    textKey = "skillsBodyHidden"

	xSkillsTrashInfo       textKey = "skillsTrashInfo"
	xSkillsTrashEmpty      textKey = "skillsTrashEmpty"
	xSkillsTrashSearch     textKey = "skillsTrashSearch"
	xSkillsTrashTotal      textKey = "skillsTrashTotal"
	xSkillsTrashFailed     textKey = "skillsTrashFailed"
	xSkillsRestore         textKey = "skillsRestore"
	xSkillsRestored        textKey = "skillsRestored"
	xSkillsRestoreFailed   textKey = "skillsRestoreFailed"
	xSkillsRestoreConflict textKey = "skillsRestoreConflict"
	xSkillsRestoreParent   textKey = "skillsRestoreParent"
	xSkillsTrashDamaged    textKey = "skillsTrashDamaged"
	xSkillsPurge           textKey = "skillsPurge"
	xSkillsPurgeTitle      textKey = "skillsPurgeTitle"
	xSkillsPurgeMsg        textKey = "skillsPurgeMessage"
	xSkillsPurged          textKey = "skillsPurged"
	xSkillsPurgeFailed     textKey = "skillsPurgeFailed"
	xSkillsEmptyTrash      textKey = "skillsEmptyTrash"
	xSkillsEmptyTrashTitle textKey = "skillsEmptyTrashTitle"
	xSkillsEmptyTrashMsg   textKey = "skillsEmptyTrashMessage"
	xSkillsEmptied         textKey = "skillsEmptied"
	xSkillsDeletedAt       textKey = "skillsDeletedAt"
	xSkillsOriginal        textKey = "skillsOriginal"
	xSkillsOpenTrashDir    textKey = "skillsOpenTrashDir"
	xSkillsKindBrokenLink  textKey = "skillsKindBrokenLink"
	xSkillsKindLink        textKey = "skillsKindLink"

	xSkillsManageInSettings textKey = "skillsManageInSettings"
)

func init() {
	for key, texts := range map[textKey][7]string{
		xSkillsSettingsTab:    {"Skills", "Skill 管理", "Skill 管理", "Skill 관리", "Skill管理", "Навыки", "Skills"},
		xSkillsTrash:          {"Trash", "回收站", "回收站", "휴지통", "ゴミ箱", "Корзина", "Corbeille"},
		xSkillsAll:            {"All", "全部", "全部", "전체", "すべて", "Все", "Tous"},
		xSkillsSourceAll:      {"All locations", "全部位置", "全部位置", "모든 위치", "すべての場所", "Все места", "Tous les emplacements"},
		xSkillsProviderAll:    {"All providers", "全部服务商", "全部服務商", "모든 공급자", "すべてのプロバイダー", "Все провайдеры", "Tous les fournisseurs"},
		xSkillsFilter:         {"Filter", "筛选", "篩選", "필터", "絞り込み", "Фильтр", "Filtrer"},
		xSkillsSearchMore:     {"Name, description, path or repository", "名称、描述、路径或仓库", "名稱、描述、路徑或儲存庫", "이름, 설명, 경로 또는 저장소", "名前、説明、パス、リポジトリ", "Имя, описание, путь или репозиторий", "Nom, description, chemin ou dépôt"},
		xSkillsFlagShared:     {"Shared", "共享", "共用", "공유", "共有", "Общие", "Partagés"},
		xSkillsFlagLink:       {"Links", "链接", "連結", "링크", "リンク", "Ссылки", "Liens"},
		xSkillsFlagGitHub:     {"GitHub", "GitHub", "GitHub", "GitHub", "GitHub", "GitHub", "GitHub"},
		xSkillsFlagIssues:     {"Has issues", "有问题", "有問題", "문제 있음", "問題あり", "С проблемами", "Avec problèmes"},
		xSkillsFlagDeletable:  {"Removable", "可删除", "可刪除", "삭제 가능", "削除可能", "Можно удалить", "Supprimables"},
		xSkillsFlagSwitchable: {"Can switch on / off", "可启停", "可啟停", "켜기/끄기 가능", "オン/オフ可能", "Можно включать", "Activables"},
		xSkillsClearFilters:   {"Clear filters", "清除筛选", "清除篩選", "필터 지우기", "絞り込みを解除", "Сбросить фильтры", "Effacer les filtres"},
		xSkillsSort:           {"Sort", "排序", "排序", "정렬", "並べ替え", "Сортировка", "Trier"},
		xSkillsSortCLI:        {"By location", "按位置", "按位置", "위치별", "場所順", "По месту", "Par emplacement"},
		xSkillsSortName:       {"Name", "名称", "名稱", "이름", "名前", "Имя", "Nom"},
		xSkillsSortModified:   {"Recently modified", "最近修改", "最近修改", "최근 수정", "最近の変更", "Недавно изменённые", "Modifiés récemment"},
		xSkillsShowing:        {"Showing %d of %d", "显示 %d / %d", "顯示 %d / %d", "%2$d개 중 %1$d개 표시", "%2$d件中%1$d件を表示", "Показано %d из %d", "%d sur %d affichés"},
		xSkillsScannedAt:      {"Scanned at %s", "扫描于 %s", "掃描於 %s", "%s에 검사함", "%sにスキャン", "Проверено в %s", "Analysé à %s"},
		xSkillsRoots:          {"Scanned locations", "扫描位置", "掃描位置", "검사한 위치", "スキャンした場所", "Проверенные места", "Emplacements analysés"},
		xSkillsRootsHint:      {"Every global folder this CLI reads, in its order", "该 CLI 按顺序读取的全部全局目录", "該 CLI 依序讀取的全部全域目錄", "이 CLI가 순서대로 읽는 모든 전역 폴더", "このCLIが順に読み込むすべてのグローバルフォルダ", "Все глобальные папки, которые читает этот CLI, по порядку", "Tous les dossiers globaux lus par ce CLI, dans l'ordre"},
		xSkillsRootMissing:    {"Not present", "不存在", "不存在", "없음", "存在しません", "Отсутствует", "Absent"},
		xSkillsCount:          {"Skills: %d", "%d 个", "%d 個", "%d개", "%d件", "Навыков: %d", "Skills : %d"},
		xSkillsShowIssues:     {"Show", "展开", "展開", "펼치기", "表示", "Показать", "Afficher"},
		xSkillsHideIssues:     {"Hide", "收起", "收合", "접기", "隠す", "Скрыть", "Masquer"},
		xSkillsIssuesHint:     {"Broken links, nested skills, duplicate names and invalid frontmatter", "失效链接、嵌套、重名与无效 frontmatter", "失效連結、巢狀、重名與無效 frontmatter", "끊어진 링크, 중첩, 중복 이름, 잘못된 frontmatter", "切れたリンク、入れ子、名前の重複、無効なfrontmatter", "Битые ссылки, вложенность, повторы имён и неверный frontmatter", "Liens cassés, imbrications, noms en double et frontmatter invalide"},
		xSkillsSharedCompact:  {"Shared: %s", "共享：%s", "共用：%s", "공유: %s", "共有：%s", "Общий: %s", "Partagé : %s"},

		xSkillsBack:                   {"Back", "返回", "返回", "뒤로", "戻る", "Назад", "Retour"},
		xSkillsOpenRepo:               {"Repository", "打开仓库", "打開儲存庫", "저장소 열기", "リポジトリを開く", "Репозиторий", "Dépôt"},
		xSkillsOpenOnline:             {"Folder in repository", "仓库中的目录", "儲存庫中的目錄", "저장소의 폴더", "リポジトリ内のフォルダ", "Папка в репозитории", "Dossier dans le dépôt"},
		xSkillsOpenHomepage:           {"Homepage", "主页", "首頁", "홈페이지", "ホームページ", "Домашняя страница", "Page d'accueil"},
		xSkillsCopyPath:               {"Copy path", "复制路径", "複製路徑", "경로 복사", "パスをコピー", "Копировать путь", "Copier le chemin"},
		xSkillsCopied:                 {"Path copied", "已复制路径", "已複製路徑", "경로를 복사했습니다", "パスをコピーしました", "Путь скопирован", "Chemin copié"},
		xSkillsTransfer:               {"Copy or move skill", "复制或移动 Skill", "複製或移動 Skill", "Skill 복사 또는 이동", "Skillをコピーまたは移動", "Копировать или переместить навык", "Copier ou déplacer le skill"},
		xSkillsCopyTo:                 {"Copy to", "复制到", "複製到", "복사 대상", "コピー先", "Копировать в", "Copier vers"},
		xSkillsMoveTo:                 {"Move to", "移动到", "移動到", "이동 대상", "移動先", "Переместить в", "Déplacer vers"},
		xSkillsMove:                   {"Move", "移动", "移動", "이동", "移動", "Переместить", "Déplacer"},
		xSkillsMoveTitle:              {"Move %s to %s?", "将 %s 移动到 %s？", "將 %s 移動到 %s？", "%s을(를) %s(으)로 이동할까요?", "%sを%sへ移動しますか？", "Переместить %s в %s?", "Déplacer %s vers %s ?"},
		xSkillsMoveMessage:            {"A copy is created in %s, then the source at %s moves to the Trash. Existing items are never overwritten.", "先在 %s 中创建副本，再将位于 %s 的原件移到回收站，不会覆盖已有内容。", "先在 %s 中建立副本，再將位於 %s 的原件移到資源回收筒，不會覆寫已有內容。", "%s에 복사본을 만든 다음 %s의 원본을 휴지통으로 이동합니다. 기존 항목은 덮어쓰지 않습니다.", "%sにコピーを作成してから、%sにある元の項目をゴミ箱へ移動します。既存の項目は上書きしません。", "Копия создаётся в %s, затем исходник из %s перемещается в корзину. Существующие объекты не перезаписываются.", "Une copie est créée dans %s, puis la source située dans %s est mise à la corbeille. Aucun élément existant n'est écrasé."},
		xSkillsCopiedTo:               {"Copied to %s", "已复制到 %s", "已複製到 %s", "%s에 복사했습니다", "%sにコピーしました", "Скопировано в %s", "Copié vers %s"},
		xSkillsMovedTo:                {"Moved to %s. %s", "已移动到 %s；%s", "已移動到 %s；%s", "%s(으)로 이동했습니다. %s", "%sに移動しました。%s", "Перемещено в %s. %s", "Déplacé vers %s. %s"},
		xSkillsTransferFailed:         {"Could not copy or move: %s", "无法复制或移动：%s", "無法複製或移動：%s", "복사하거나 이동할 수 없습니다: %s", "コピーまたは移動できません：%s", "Не удалось скопировать или переместить: %s", "Impossible de copier ou déplacer : %s"},
		xSkillsTransferProviderError:  {"the target CLI is not supported", "不支持目标 CLI", "不支援目標 CLI", "대상 CLI를 지원하지 않습니다", "対象CLIはサポートされていません", "целевой CLI не поддерживается", "le CLI cible n'est pas pris en charge"},
		xSkillsTransferConflict:       {"a skill with the same folder name already exists at the target", "目标位置已有同名 Skill 目录", "目標位置已有同名 Skill 目錄", "대상에 같은 이름의 Skill 폴더가 이미 있습니다", "移動先に同名のSkillフォルダがあります", "в целевом месте уже есть папка навыка с таким именем", "un dossier de skill du même nom existe déjà à destination"},
		xSkillsTransferSource:         {"the source changed or is no longer valid", "源 Skill 已变化或不再有效", "來源 Skill 已變更或不再有效", "원본 Skill이 변경되었거나 더 이상 유효하지 않습니다", "元のSkillが変更されたか、無効になりました", "исходный навык изменился или больше недействителен", "le skill source a changé ou n'est plus valide"},
		xSkillsTransferOverlap:        {"the source and target locations overlap", "源位置与目标位置重叠", "來源位置與目標位置重疊", "원본 위치와 대상 위치가 겹칩니다", "コピー元とコピー先の場所が重複しています", "исходное и целевое расположения пересекаются", "les emplacements source et cible se chevauchent"},
		xSkillsTransferUnsafeLink:     {"the skill contains a symbolic link that cannot be copied safely", "Skill 包含无法安全复制的符号链接", "Skill 包含無法安全複製的符號連結", "Skill에 안전하게 복사할 수 없는 심볼릭 링크가 있습니다", "Skillに安全にコピーできないシンボリックリンクがあります", "навык содержит символическую ссылку, которую нельзя безопасно скопировать", "le skill contient un lien symbolique qui ne peut pas être copié en toute sécurité"},
		xSkillsTransferNotMovable:     {"this source cannot be moved as one skill; it can still be copied", "此来源无法作为单个 Skill 移动，但仍可复制", "此來源無法作為單一 Skill 移動，但仍可複製", "이 원본은 단일 Skill로 이동할 수 없지만 복사는 가능합니다", "このコピー元は単一のSkillとして移動できませんが、コピーはできます", "этот источник нельзя переместить как один навык, но его можно скопировать", "cette source ne peut pas être déplacée comme un seul skill, mais elle peut être copiée"},
		xSkillsTransferPartial:        {"the copy was created at %s, but the source remains because it could not be moved to the Trash", "副本已创建在 %s，但原件无法移到回收站，仍保留在原处", "副本已建立在 %s，但原件無法移到資源回收筒，仍保留在原處", "복사본이 %s에 생성되었지만 원본을 휴지통으로 이동하지 못해 원래 위치에 남아 있습니다", "コピーは%sに作成されましたが、元の項目をゴミ箱へ移動できず元の場所に残っています", "копия создана в %s, но исходник остался на месте, поскольку его не удалось переместить в корзину", "la copie a été créée dans %s, mais la source reste en place car elle n'a pas pu être mise à la corbeille"},
		xSkillsTransferPartialUnknown: {"the copy was created, but the source remains because it could not be moved to the Trash", "副本已创建，但原件无法移到回收站，仍保留在原处", "副本已建立，但原件無法移到資源回收筒，仍保留在原處", "복사본은 생성되었지만 원본을 휴지통으로 이동하지 못해 원래 위치에 남아 있습니다", "コピーは作成されましたが、元の項目をゴミ箱へ移動できず元の場所に残っています", "копия создана, но исходник остался на месте, поскольку его не удалось переместить в корзину", "la copie a été créée, mais la source reste en place car elle n'a pas pu être mise à la corbeille"},
		xSkillsMovedTrash:             {"Moved to the CodexRunway trash; restore it from the Trash tab", "已移到 CodexRunway 回收站，可在“回收站”中恢复", "已移到 CodexRunway 回收站，可在「回收站」中復原", "CodexRunway 휴지통으로 옮겼습니다. 휴지통 탭에서 복원할 수 있습니다", "CodexRunwayのゴミ箱に移動しました。ゴミ箱タブから復元できます", "Перемещено в корзину CodexRunway; восстановить можно на вкладке «Корзина»", "Déplacé dans la corbeille de CodexRunway ; restaurez-le depuis l'onglet Corbeille"},
		xSkillsMovedSystem:            {"Moved to the system Trash, as it is on another volume", "位于其他磁盘，已移到系统回收站", "位於其他磁碟，已移到系統資源回收筒", "다른 볼륨에 있어 시스템 휴지통으로 옮겼습니다", "別のボリュームにあるため、システムのゴミ箱に移動しました", "Перемещено в системную корзину: файл на другом томе", "Déplacé dans la corbeille du système, car sur un autre volume"},
		xSkillsReason:                 {"Reason", "原因", "原因", "이유", "理由", "Причина", "Raison"},
		xSkillsSwitch:                 {"On / off", "启停", "啟停", "켜기/끄기", "オン/オフ", "Вкл. / выкл.", "Activation"},
		xSkillsSecBasics:              {"Details", "基本信息", "基本資訊", "기본 정보", "基本情報", "Сведения", "Détails"},
		xSkillsDisplayName:            {"Display name", "显示名称", "顯示名稱", "표시 이름", "表示名", "Отображаемое имя", "Nom affiché"},
		xSkillsScope:                  {"Type", "类型", "類型", "유형", "種類", "Тип", "Type"},
		xSkillsScopeUser:              {"User", "用户", "使用者", "사용자", "ユーザー", "Пользовательский", "Utilisateur"},
		xSkillsRealPath:               {"Real path", "真实路径", "實際路徑", "실제 경로", "実際のパス", "Реальный путь", "Chemin réel"},

		xSkillsSecOrigin:     {"Source and install", "来源与安装", "來源與安裝", "출처 및 설치", "入手元とインストール", "Источник и установка", "Source et installation"},
		xSkillsOriginUnknown: {"No repository, install record or homepage was found for this skill", "未找到该 Skill 的仓库、安装记录或主页", "未找到該 Skill 的儲存庫、安裝紀錄或首頁", "이 Skill의 저장소, 설치 기록, 홈페이지를 찾지 못했습니다", "このSkillのリポジトリ、インストール記録、ホームページは見つかりません", "Репозиторий, запись об установке и домашняя страница не найдены", "Aucun dépôt, historique d'installation ni page d'accueil trouvé"},
		xSkillsInstaller:     {"Installed by", "安装方式", "安裝方式", "설치 방식", "インストール方法", "Способ установки", "Installé via"},
		xSkillsInstSkillsCLI: {"skills CLI (~/.agents/.skill-lock.json)", "skills CLI（~/.agents/.skill-lock.json）", "skills CLI（~/.agents/.skill-lock.json）", "skills CLI (~/.agents/.skill-lock.json)", "skills CLI（~/.agents/.skill-lock.json）", "skills CLI (~/.agents/.skill-lock.json)", "skills CLI (~/.agents/.skill-lock.json)"},
		xSkillsInstGit:       {"Git clone", "Git 克隆", "Git 複製", "Git 클론", "Gitクローン", "Клон Git", "Clone Git"},
		xSkillsInstMarket:    {"Plugin from marketplace %s", "插件市场 %s", "外掛市集 %s", "마켓플레이스 %s의 플러그인", "マーケットプレイス %s のプラグイン", "Плагин из каталога %s", "Plugin du catalogue %s"},
		xSkillsInstFront:     {"Declared in SKILL.md", "SKILL.md 中声明", "SKILL.md 中宣告", "SKILL.md에 명시", "SKILL.mdに記載", "Указано в SKILL.md", "Indiqué dans SKILL.md"},
		xSkillsRepository:    {"Repository", "仓库", "儲存庫", "저장소", "リポジトリ", "Репозиторий", "Dépôt"},
		xSkillsRepoPath:      {"Path in repository", "仓库内路径", "儲存庫內路徑", "저장소 내 경로", "リポジトリ内のパス", "Путь в репозитории", "Chemin dans le dépôt"},
		xSkillsBranch:        {"Branch", "分支", "分支", "브랜치", "ブランチ", "Ветка", "Branche"},
		xSkillsHomepage:      {"Homepage", "主页", "首頁", "홈페이지", "ホームページ", "Домашняя страница", "Page d'accueil"},
		xSkillsVersion:       {"Version", "版本", "版本", "버전", "バージョン", "Версия", "Version"},
		xSkillsInstalledAt:   {"Installed", "安装时间", "安裝時間", "설치 시각", "インストール日時", "Установлен", "Installé le"},
		xSkillsUpdatedAt:     {"Updated", "更新时间", "更新時間", "업데이트 시각", "更新日時", "Обновлён", "Mis à jour le"},

		xSkillsSecFiles:      {"Files", "文件", "檔案", "파일", "ファイル", "Файлы", "Fichiers"},
		xSkillsFilesCount:    {"%d files, %d folders", "%d 个文件，%d 个文件夹", "%d 個檔案，%d 個資料夾", "파일 %d개, 폴더 %d개", "ファイル%d件、フォルダ%d件", "Файлов: %d, папок: %d", "%d fichiers, %d dossiers"},
		xSkillsFilesPartial:  {"limit reached, partly listed", "已达上限，仅列出部分", "已達上限，僅列出部分", "한도에 도달해 일부만 표시", "上限に達したため一部のみ", "достигнут лимит, показана часть", "limite atteinte, liste partielle"},
		xSkillsFilesMore:     {"Show all %d", "显示全部 %d 项", "顯示全部 %d 項", "%d개 모두 보기", "%d件すべて表示", "Показать все (%d)", "Tout afficher (%d)"},
		xSkillsFilesLess:     {"Show fewer", "收起", "收合", "접기", "折りたたむ", "Свернуть", "Réduire"},
		xSkillsNoFiles:       {"No files", "没有文件", "沒有檔案", "파일 없음", "ファイルはありません", "Файлов нет", "Aucun fichier"},
		xSkillsSkippedDir:    {"Not listed", "未展开", "未展開", "펼치지 않음", "展開しない", "Не раскрыто", "Non listé"},
		xSkillsInspectFailed: {"Could not list the folder: %s", "无法列出目录：%s", "無法列出目錄：%s", "폴더를 나열할 수 없습니다: %s", "フォルダを一覧できません：%s", "Не удалось прочитать папку: %s", "Impossible de lister le dossier : %s"},
		xSkillsFrontHint:     {"Fields in the header of SKILL.md", "SKILL.md 头部的字段", "SKILL.md 標頭的欄位", "SKILL.md 머리말의 필드", "SKILL.md ヘッダーのフィールド", "Поля заголовка SKILL.md", "Champs de l'en-tête de SKILL.md"},
		xSkillsBodyHint:      {"Read on demand, up to 256 KB", "按需读取，最多 256 KB", "按需讀取，最多 256 KB", "필요할 때 최대 256 KB 읽기", "必要時に最大256 KBを読み込み", "Читается по запросу, до 256 КБ", "Lu à la demande, jusqu'à 256 Ko"},
		xSkillsBodyHidden:    {"Not read yet", "尚未读取", "尚未讀取", "아직 읽지 않음", "未読み込み", "Ещё не прочитан", "Pas encore lu"},

		xSkillsTrashInfo:       {"Deleted skills stay in ~/.codex-runway/skills-trash until you restore them. Deleting one permanently moves it to the system Trash.", "删除的 Skill 会保留在 ~/.codex-runway/skills-trash，可随时恢复；彻底删除会把它移到系统回收站。", "刪除的 Skill 會保留在 ~/.codex-runway/skills-trash，可隨時復原；徹底刪除會把它移到系統資源回收筒。", "삭제한 Skill은 ~/.codex-runway/skills-trash에 남아 언제든 복원할 수 있으며, 완전히 삭제하면 시스템 휴지통으로 옮겨집니다.", "削除したSkillは ~/.codex-runway/skills-trash に残り、いつでも復元できます。完全に削除するとシステムのゴミ箱に移動します。", "Удалённые навыки хранятся в ~/.codex-runway/skills-trash и могут быть восстановлены; окончательное удаление перемещает их в системную корзину.", "Les skills supprimés restent dans ~/.codex-runway/skills-trash et peuvent être restaurés ; la suppression définitive les déplace dans la corbeille du système."},
		xSkillsTrashEmpty:      {"The trash is empty", "回收站为空", "回收站是空的", "휴지통이 비어 있습니다", "ゴミ箱は空です", "Корзина пуста", "La corbeille est vide"},
		xSkillsTrashSearch:     {"Search the trash", "搜索回收站", "搜尋回收站", "휴지통 검색", "ゴミ箱を検索", "Поиск в корзине", "Rechercher dans la corbeille"},
		xSkillsTrashTotal:      {"%d items · %s", "%d 项 · %s", "%d 項 · %s", "%d개 · %s", "%d件 · %s", "Объектов: %d · %s", "%d éléments · %s"},
		xSkillsTrashFailed:     {"Could not read the trash: %s", "无法读取回收站：%s", "無法讀取回收站：%s", "휴지통을 읽을 수 없습니다: %s", "ゴミ箱を読み込めません：%s", "Не удалось прочитать корзину: %s", "Impossible de lire la corbeille : %s"},
		xSkillsRestore:         {"Restore", "恢复", "復原", "복원", "復元", "Восстановить", "Restaurer"},
		xSkillsRestored:        {"Restored to %s", "已恢复到 %s", "已復原到 %s", "%s(으)로 복원했습니다", "%s に復元しました", "Восстановлено в %s", "Restauré dans %s"},
		xSkillsRestoreFailed:   {"Could not restore: %s", "无法恢复：%s", "無法復原：%s", "복원할 수 없습니다: %s", "復元できません：%s", "Не удалось восстановить: %s", "Impossible de restaurer : %s"},
		xSkillsRestoreConflict: {"something already exists at the original location", "原位置已有同名内容", "原位置已有同名內容", "원래 위치에 같은 이름의 항목이 있습니다", "元の場所に同じ名前の項目があります", "в исходном месте уже что-то есть", "un élément existe déjà à l'emplacement d'origine"},
		xSkillsRestoreParent:   {"the original parent folder no longer exists", "原上级目录已不存在", "原上層目錄已不存在", "원래 상위 폴더가 더 이상 없습니다", "元の親フォルダがもうありません", "исходной родительской папки больше нет", "le dossier parent d'origine n'existe plus"},
		xSkillsTrashDamaged:    {"this trash entry is damaged", "该回收站条目已损坏", "該回收站項目已損壞", "이 휴지통 항목이 손상되었습니다", "このゴミ箱の項目は破損しています", "запись корзины повреждена", "cette entrée de la corbeille est endommagée"},
		xSkillsPurge:           {"Delete permanently", "彻底删除", "徹底刪除", "완전히 삭제", "完全に削除", "Удалить навсегда", "Supprimer définitivement"},
		xSkillsPurgeTitle:      {"Delete permanently?", "彻底删除？", "徹底刪除？", "완전히 삭제할까요?", "完全に削除しますか？", "Удалить навсегда?", "Supprimer définitivement ?"},
		xSkillsPurgeMsg:        {"It moves to the system Trash and can no longer be restored here.", "它会移到系统回收站，之后无法在这里恢复。", "它會移到系統資源回收筒，之後無法在這裡復原。", "시스템 휴지통으로 옮겨지며 여기서는 더 이상 복원할 수 없습니다.", "システムのゴミ箱に移動し、ここからは復元できなくなります。", "Объект переместится в системную корзину, и восстановить его здесь будет нельзя.", "Il sera déplacé dans la corbeille du système et ne pourra plus être restauré ici."},
		xSkillsPurged:          {"Moved to the system Trash", "已移到系统回收站", "已移到系統資源回收筒", "시스템 휴지통으로 옮겼습니다", "システムのゴミ箱に移動しました", "Перемещено в системную корзину", "Déplacé dans la corbeille du système"},
		xSkillsPurgeFailed:     {"Could not move to the system Trash: %s", "无法移到系统回收站：%s", "無法移到系統資源回收筒：%s", "시스템 휴지통으로 옮길 수 없습니다: %s", "システムのゴミ箱に移動できません：%s", "Не удалось переместить в системную корзину: %s", "Impossible de déplacer dans la corbeille du système : %s"},
		xSkillsEmptyTrash:      {"Empty trash", "清空回收站", "清空回收站", "휴지통 비우기", "ゴミ箱を空にする", "Очистить корзину", "Vider la corbeille"},
		xSkillsEmptyTrashTitle: {"Empty the trash?", "清空回收站？", "清空回收站？", "휴지통을 비울까요?", "ゴミ箱を空にしますか？", "Очистить корзину?", "Vider la corbeille ?"},
		xSkillsEmptyTrashMsg:   {"All %d items move to the system Trash.", "全部 %d 项会移到系统回收站。", "全部 %d 項會移到系統資源回收筒。", "%d개 항목이 모두 시스템 휴지통으로 옮겨집니다.", "%d件すべてがシステムのゴミ箱に移動します。", "Все объекты (%d) переместятся в системную корзину.", "Les %d éléments seront déplacés dans la corbeille du système."},
		xSkillsEmptied:         {"Moved %d items to the system Trash", "已将 %d 项移到系统回收站", "已將 %d 項移到系統資源回收筒", "%d개 항목을 시스템 휴지통으로 옮겼습니다", "%d件をシステムのゴミ箱に移動しました", "В системную корзину перемещено: %d", "%d éléments déplacés dans la corbeille du système"},
		xSkillsDeletedAt:       {"Deleted", "删除于", "刪除於", "삭제 시각", "削除日時", "Удалено", "Supprimé le"},
		xSkillsOriginal:        {"From", "原位置", "原位置", "원래 위치", "元の場所", "Откуда", "Origine"},
		xSkillsOpenTrashDir:    {"Open the trash folder", "打开回收站目录", "打開回收站目錄", "휴지통 폴더 열기", "ゴミ箱フォルダを開く", "Открыть папку корзины", "Ouvrir le dossier de la corbeille"},
		xSkillsKindBrokenLink:  {"Broken link", "失效链接", "失效連結", "끊어진 링크", "切れたリンク", "Битая ссылка", "Lien cassé"},
		xSkillsKindLink:        {"Link", "链接", "連結", "링크", "リンク", "Ссылка", "Lien"},

		xSkillsManageInSettings: {"Manage in Settings", "在设置中管理", "在設定中管理", "설정에서 관리", "設定で管理", "Управлять в настройках", "Gérer dans les réglages"},
	} {
		extraTexts[key] = texts
	}
}
