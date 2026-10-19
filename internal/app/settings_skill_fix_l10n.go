package app

const (
	xSkillsFix              textKey = "skillsFix"
	xSkillsFixTitle         textKey = "skillsFixTitle"
	xSkillsFixFailed        textKey = "skillsFixFailed"
	xSkillsFixRisks         textKey = "skillsFixRisks"
	xSkillsFixLift          textKey = "skillsFixLift"
	xSkillsFixLiftMsg       textKey = "skillsFixLiftMessage"
	xSkillsFixDisable       textKey = "skillsFixDisable"
	xSkillsFixDisableMsg    textKey = "skillsFixDisableMessage"
	xSkillsFixTrash         textKey = "skillsFixTrash"
	xSkillsFixTrashMsg      textKey = "skillsFixTrashMessage"
	xSkillsFixDesc          textKey = "skillsFixDescription"
	xSkillsFixDescMsg       textKey = "skillsFixDescriptionMessage"
	xSkillsFixDescLabel     textKey = "skillsFixDescriptionLabel"
	xSkillsRiskWrapper      textKey = "skillsRiskWrapper"
	xSkillsRiskWrapperOnly  textKey = "skillsRiskWrapperOnly"
	xSkillsRiskWrapperLink  textKey = "skillsRiskWrapperLink"
	xSkillsRiskReinstall    textKey = "skillsRiskReinstall"
	xSkillsRiskEditBackup   textKey = "skillsRiskEditBackup"
	xSkillsRiskEditUpdate   textKey = "skillsRiskEditUpdate"
	xSkillsRiskEditShared   textKey = "skillsRiskEditShared"
	xSkillsRiskEditLink     textKey = "skillsRiskEditLink"
	xSkillsRiskConfigBackup textKey = "skillsRiskConfigBackup"
	xSkillsFixedLift        textKey = "skillsFixedLift"
	xSkillsFixedDisable     textKey = "skillsFixedDisable"
	xSkillsFixedDesc        textKey = "skillsFixedDescription"
	xSkillsFixStale         textKey = "skillsFixStale"
	xSkillsRiskRestart      textKey = "skillsRiskRestart"
	xSkillsFixLiftAllMsg    textKey = "skillsFixLiftAllMessage"
	xSkillsFixedLiftAll     textKey = "skillsFixedLiftAll"
	xSkillsSelectOne        textKey = "skillsSelectOne"
	xSkillsSelectFiltered   textKey = "skillsSelectFiltered"
	xSkillsClearSelection   textKey = "skillsClearSelection"
	xSkillsSelectionCounts  textKey = "skillsSelectionCounts"
	xSkillsFixSelected      textKey = "skillsFixSelected"
	xSkillsDeleteSelected   textKey = "skillsDeleteSelected"
	xSkillsBatchFixTitle    textKey = "skillsBatchFixTitle"
	xSkillsBatchFixMsg      textKey = "skillsBatchFixMessage"
	xSkillsBatchDescNote    textKey = "skillsBatchDescriptionNote"
	xSkillsBatchSkipFix     textKey = "skillsBatchSkipFix"
	xSkillsBatchDeleteTitle textKey = "skillsBatchDeleteTitle"
	xSkillsBatchDeleteMsg   textKey = "skillsBatchDeleteMessage"
	xSkillsBatchSkipDelete  textKey = "skillsBatchSkipDelete"
	xSkillsBatchFixed       textKey = "skillsBatchFixed"
	xSkillsBatchDeleted     textKey = "skillsBatchDeleted"
	xSkillsBatchFailed      textKey = "skillsBatchFailed"
	xSkillsBatchPartial     textKey = "skillsBatchPartial"
)

func init() {
	for key, texts := range map[textKey][7]string{
		xSkillsFix:              {"Fix", "修复", "修復", "수정", "修正", "Исправить", "Corriger"},
		xSkillsFixTitle:         {"Fix: %s", "修复：%s", "修復：%s", "수정: %s", "修正：%s", "Исправить: %s", "Corriger : %s"},
		xSkillsFixFailed:        {"Could not fix: %s", "修复失败：%s", "修復失敗：%s", "수정하지 못했습니다: %s", "修正できませんでした：%s", "Не удалось исправить: %s", "Impossible de corriger : %s"},
		xSkillsFixRisks:         {"Before you continue", "继续前请了解", "繼續前請了解", "계속하기 전에 확인하세요", "続行する前に", "Перед продолжением", "Avant de continuer"},
		xSkillsFixLift:          {"Move to the top", "移到顶层", "移到頂層", "최상위로 이동", "最上位へ移動", "Поднять наверх", "Remonter au premier niveau"},
		xSkillsFixLiftMsg:       {"Claude Code only reads skills one level under its skills folder. %s moves to %s.", "Claude Code 只读取 skills 目录下一级的 skill。%s 将移到 %s。", "Claude Code 只讀取 skills 目錄下一層的 skill。%s 將移到 %s。", "Claude Code는 skills 폴더 바로 아래의 skill만 읽습니다. %s을(를) %s(으)로 옮깁니다.", "Claude Code は skills フォルダ直下の skill だけを読み込みます。%s を %s に移動します。", "Claude Code читает навыки только на один уровень ниже папки skills. %s будет перемещён в %s.", "Claude Code ne lit que les skills situés juste sous son dossier skills. %s sera déplacé vers %s."},
		xSkillsFixDisable:       {"Turn this copy off", "停用此副本", "停用此副本", "이 사본 끄기", "このコピーをオフ", "Отключить эту копию", "Désactiver cette copie"},
		xSkillsFixDisableMsg:    {"Turns off %s by its path in Codex config.toml and keeps %s.", "在 Codex 的 config.toml 中按路径停用 %s，保留 %s。", "在 Codex 的 config.toml 中依路徑停用 %s，保留 %s。", "Codex config.toml에서 경로로 %s을(를) 끄고 %s은(는) 유지합니다.", "Codex の config.toml でパスを指定して %s をオフにし、%s を残します。", "Отключает %s по пути в config.toml Codex и оставляет %s.", "Désactive %s par son chemin dans config.toml de Codex et conserve %s."},
		xSkillsFixTrash:         {"Move this copy to the trash", "将此副本移入回收站", "將此副本移入回收站", "이 사본을 휴지통으로", "このコピーをゴミ箱へ", "Переместить копию в корзину", "Mettre cette copie à la corbeille"},
		xSkillsFixTrashMsg:      {"%s goes to the CodexRunway skills trash; %s stays.", "%s 将移入 CodexRunway 的 Skill 回收站，保留 %s。", "%s 將移入 CodexRunway 的 Skill 回收站，保留 %s。", "%s은(는) CodexRunway Skill 휴지통으로 이동하고 %s은(는) 유지됩니다.", "%s を CodexRunway の Skill ゴミ箱に移動し、%s を残します。", "%s переместится в корзину навыков CodexRunway, %s останется.", "%s ira dans la corbeille de skills de CodexRunway ; %s est conservé."},
		xSkillsFixDesc:          {"Add description", "写入描述", "寫入描述", "설명 추가", "説明を追加", "Добавить описание", "Ajouter la description"},
		xSkillsFixDescMsg:       {"Adds a description to the frontmatter of %s; the CLI uses it to decide when to use the skill.", "在 %s 的 frontmatter 中写入 description，CLI 依据它判断何时使用此 skill。", "在 %s 的 frontmatter 中寫入 description，CLI 依據它判斷何時使用此 skill。", "%s의 frontmatter에 description을 추가합니다. CLI는 이를 보고 skill 사용 시점을 판단합니다.", "%s の frontmatter に description を追加します。CLI はこれを基に skill を使うタイミングを判断します。", "Добавляет description во frontmatter %s; по нему CLI решает, когда использовать навык.", "Ajoute une description au frontmatter de %s ; le CLI s’en sert pour décider quand utiliser le skill."},
		xSkillsFixDescLabel:     {"Description", "描述", "描述", "설명", "説明", "Описание", "Description"},
		xSkillsRiskWrapper:      {"The outer folder %s goes to the skills trash with its %d other items (such as README or .git); you can restore it from the trash.", "外层文件夹 %s 连同其中另外 %d 项（如 README、.git）会一并移入 Skill 回收站，可在回收站恢复。", "外層資料夾 %s 連同其中另外 %d 項（如 README、.git）會一併移入 Skill 回收站，可在回收站復原。", "바깥 폴더 %s이(가) 다른 항목 %d개(README, .git 등)와 함께 Skill 휴지통으로 이동하며, 휴지통에서 복원할 수 있습니다.", "外側のフォルダ %s は、ほかの %d 項目（README や .git など）とともに Skill ゴミ箱に移動します。ゴミ箱から復元できます。", "Внешняя папка %s вместе с ещё %d объектами (например, README или .git) попадёт в корзину навыков; её можно восстановить.", "Le dossier extérieur %s ira dans la corbeille de skills avec ses %d autres éléments (README, .git…) ; vous pourrez le restaurer."},
		xSkillsRiskWrapperOnly:  {"The outer folder %s goes to the skills trash; you can restore it from the trash.", "外层文件夹 %s 会移入 Skill 回收站，可在回收站恢复。", "外層資料夾 %s 會移入 Skill 回收站，可在回收站復原。", "바깥 폴더 %s이(가) Skill 휴지통으로 이동하며, 휴지통에서 복원할 수 있습니다.", "外側のフォルダ %s は Skill ゴミ箱に移動します。ゴミ箱から復元できます。", "Внешняя папка %s попадёт в корзину навыков; её можно восстановить.", "Le dossier extérieur %s ira dans la corbeille de skills ; vous pourrez le restaurer."},
		xSkillsRiskWrapperLink:  {"%s is a link: only the link moves to the trash, and the folder it points to stays as it is.", "%s 是链接：只把链接移入回收站，它指向的文件夹保持不变。", "%s 是連結：只把連結移入回收站，它指向的資料夾保持不變。", "%s은(는) 링크입니다. 링크만 휴지통으로 이동하고 가리키는 폴더는 그대로 둡니다.", "%s はリンクです。リンクだけをゴミ箱に移動し、リンク先のフォルダはそのまま残ります。", "%s — это ссылка: в корзину попадёт только она, папка, на которую она указывает, не изменится.", "%s est un lien : seul le lien va à la corbeille, le dossier qu’il désigne reste intact."},
		xSkillsRiskReinstall:    {"If it was installed with git or the skills CLI, updating it the same way will no longer work; reinstall it to keep it updated.", "如果它是用 git 或 skills CLI 安装的，之后无法再按原方式更新，需要重新安装。", "如果它是用 git 或 skills CLI 安裝的，之後無法再依原方式更新，需要重新安裝。", "git이나 skills CLI로 설치했다면 같은 방식으로 더 이상 업데이트할 수 없으니 다시 설치해야 합니다.", "git や skills CLI でインストールした場合、同じ方法では更新できなくなるため、再インストールが必要です。", "Если навык установлен через git или skills CLI, обновлять его прежним способом не получится — переустановите его.", "S’il a été installé avec git ou le CLI skills, il ne pourra plus être mis à jour ainsi ; réinstallez-le."},
		xSkillsRiskEditBackup:   {"This changes the skill’s own SKILL.md; a copy is saved to ~/.codex-runway/backups first.", "这会修改 skill 自身的 SKILL.md，修改前会先备份到 ~/.codex-runway/backups。", "這會修改 skill 自身的 SKILL.md，修改前會先備份到 ~/.codex-runway/backups。", "skill 자체의 SKILL.md를 수정하며, 먼저 ~/.codex-runway/backups에 백업합니다.", "skill 自体の SKILL.md を変更します。変更前に ~/.codex-runway/backups にバックアップします。", "Изменяется сам SKILL.md навыка; перед этим копия сохраняется в ~/.codex-runway/backups.", "Le SKILL.md du skill est modifié ; une copie est d’abord enregistrée dans ~/.codex-runway/backups."},
		xSkillsRiskEditUpdate:   {"If git or the skills CLI manages it, the next update may overwrite this change or conflict with it.", "如果它由 git 或 skills CLI 管理，下次更新可能覆盖此修改或产生冲突。", "如果它由 git 或 skills CLI 管理，下次更新可能覆蓋此修改或產生衝突。", "git이나 skills CLI가 관리한다면 다음 업데이트에서 이 변경이 덮어써지거나 충돌할 수 있습니다.", "git や skills CLI で管理されている場合、次の更新でこの変更が上書きされるか、競合する可能性があります。", "Если навыком управляет git или skills CLI, следующее обновление может перезаписать изменение или вызвать конфликт.", "Si git ou le CLI skills le gère, la prochaine mise à jour pourra écraser cette modification ou entrer en conflit."},
		xSkillsRiskEditShared:   {"%s also reads this file and sees the change too.", "%s 也读取此文件，修改对它同样生效。", "%s 也讀取此檔案，修改對它同樣生效。", "%s도 이 파일을 읽으므로 변경 사항이 함께 적용됩니다.", "%s もこのファイルを読み込むため、変更が同様に反映されます。", "%s тоже читает этот файл и увидит изменение.", "%s lit aussi ce fichier et verra la modification."},
		xSkillsRiskEditLink:     {"The skill is reached through a link; the change is written to the file the link points to.", "此 skill 经链接放置，修改会写入链接指向的原文件。", "此 skill 經連結放置，修改會寫入連結指向的原檔案。", "이 skill은 링크로 연결되어 있어 링크가 가리키는 원본 파일이 수정됩니다.", "この skill はリンク経由で置かれているため、リンク先の元ファイルが変更されます。", "Навык подключён ссылкой; изменение запишется в файл, на который она указывает.", "Le skill est atteint par un lien ; la modification est écrite dans le fichier visé."},
		xSkillsRiskConfigBackup: {"config.toml is backed up before the change; you can turn the copy back on from the list at any time.", "写入前会备份 config.toml，之后可随时在列表中重新启用此副本。", "寫入前會備份 config.toml，之後可隨時在清單中重新啟用此副本。", "변경 전에 config.toml을 백업하며, 언제든 목록에서 이 사본을 다시 켤 수 있습니다.", "変更前に config.toml をバックアップします。一覧からいつでもこのコピーをオンに戻せます。", "Перед изменением config.toml сохраняется копия; копию можно снова включить в списке.", "config.toml est sauvegardé avant la modification ; vous pourrez réactiver la copie depuis la liste."},
		xSkillsFixedLift:        {"Moved to %s; the outer folder is in the skills trash. Restart Claude Code to load it", "已移到 %s，外层文件夹已移入 Skill 回收站；重启 Claude Code 后生效", "已移到 %s，外層資料夾已移入 Skill 回收站；重新啟動 Claude Code 後生效", "%s(으)로 옮겼고 바깥 폴더는 Skill 휴지통에 있습니다. Claude Code를 다시 시작하면 적용됩니다", "%s に移動し、外側のフォルダは Skill ゴミ箱に移しました。Claude Code を再起動すると反映されます", "Перемещено в %s, внешняя папка — в корзине навыков. Перезапустите Claude Code", "Déplacé vers %s ; le dossier extérieur est dans la corbeille de skills. Redémarrez Claude Code"},
		xSkillsFixedDisable:     {"Turned this copy off; restart Codex to apply", "已停用此副本；重启 Codex 后生效", "已停用此副本；重新啟動 Codex 後生效", "이 사본을 껐습니다. Codex를 다시 시작하면 적용됩니다", "このコピーをオフにしました。Codex を再起動すると反映されます", "Копия отключена; перезапустите Codex", "Copie désactivée ; redémarrez Codex pour l’appliquer"},
		xSkillsFixedDesc:        {"Added the description and backed up the original; restart the CLI to apply", "已写入描述并备份原文件；重启 CLI 后生效", "已寫入描述並備份原檔案；重新啟動 CLI 後生效", "설명을 추가하고 원본을 백업했습니다. CLI를 다시 시작하면 적용됩니다", "説明を追加し、元のファイルをバックアップしました。CLI を再起動すると反映されます", "Описание добавлено, оригинал сохранён; перезапустите CLI", "Description ajoutée et original sauvegardé ; redémarrez le CLI"},
		xSkillsFixStale:         {"this problem can no longer be fixed this way; refresh and try again", "当前情况已无法这样修复，请刷新后重试", "目前情況已無法這樣修復，請重新整理後再試", "더 이상 이 방법으로 수정할 수 없습니다. 새로 고친 뒤 다시 시도하세요", "この方法ではもう修正できません。更新してからやり直してください", "так исправить уже нельзя; обновите список и повторите", "ce problème ne peut plus être corrigé ainsi ; actualisez et réessayez"},
		xSkillsRiskRestart:      {"A CLI that is already running picks the change up only after it restarts.", "已在运行的 CLI 需要重启后才会读到这项改动。", "已在執行的 CLI 需要重新啟動後才會讀到這項變更。", "이미 실행 중인 CLI는 다시 시작해야 이 변경 사항을 읽습니다.", "実行中の CLI は再起動するまでこの変更を読み込みません。", "Уже запущенный CLI увидит изменение только после перезапуска.", "Un CLI déjà lancé ne verra la modification qu’après son redémarrage."},
		xSkillsFixLiftAllMsg:    {"Claude Code only reads skills one level under its skills folder. The %[2]d skills in %[1]s move to the top of it together: %[3]s.", "Claude Code 只读取 skills 目录下一级的 skill。%[1]s 中的 %[2]d 个 skill 将一起移到 skills 目录顶层：%[3]s。", "Claude Code 只讀取 skills 目錄下一層的 skill。%[1]s 中的 %[2]d 個 skill 將一起移到 skills 目錄頂層：%[3]s。", "Claude Code는 skills 폴더 바로 아래의 skill만 읽습니다. %[1]s 안의 skill %[2]d개를 skills 폴더 최상위로 함께 옮깁니다: %[3]s.", "Claude Code は skills フォルダ直下の skill だけを読み込みます。%[1]s 内の %[2]d 個の skill をまとめて skills フォルダの最上位に移動します：%[3]s。", "Claude Code читает навыки только на один уровень ниже папки skills. Навыки из %[1]s (%[2]d) вместе переместятся наверх: %[3]s.", "Claude Code ne lit que les skills situés un niveau sous son dossier skills. Les %[2]d skills de %[1]s remontent ensemble au premier niveau : %[3]s."},
		xSkillsFixedLiftAll:     {"Moved %d skills to the top; the outer folder is in the skills trash. Restart Claude Code to load them", "已将 %d 个 skill 移到顶层，外层文件夹已移入 Skill 回收站；重启 Claude Code 后生效", "已將 %d 個 skill 移到頂層，外層資料夾已移入 Skill 回收站；重新啟動 Claude Code 後生效", "skill %d개를 최상위로 옮겼고 바깥 폴더는 Skill 휴지통에 있습니다. Claude Code를 다시 시작하면 적용됩니다", "%d 個の skill を最上位に移動し、外側のフォルダは Skill ゴミ箱に移しました。Claude Code を再起動すると反映されます", "Навыков перемещено наверх: %d, внешняя папка — в корзине навыков. Перезапустите Claude Code", "%d skills remontés au premier niveau ; le dossier extérieur est dans la corbeille de skills. Redémarrez Claude Code pour les charger"},
		xSkillsSelectOne:        {"Select %s", "选择 %s", "選擇 %s", "%s 선택", "%s を選択", "Выбрать %s", "Sélectionner %s"},
		xSkillsSelectFiltered:   {"Select filtered (%d)", "全选筛选结果（%d）", "全選篩選結果（%d）", "필터 결과 선택 (%d)", "絞り込み結果を選択（%d）", "Выбрать результаты (%d)", "Sélectionner les résultats (%d)"},
		xSkillsClearSelection:   {"Clear selection", "清除选择", "清除選擇", "선택 지우기", "選択を解除", "Снять выделение", "Effacer la sélection"},
		xSkillsSelectionCounts:  {"%d shown · %d selected", "显示 %d 个 · 已选择 %d 个", "顯示 %d 個 · 已選擇 %d 個", "%d개 표시 · %d개 선택", "%d 件表示 · %d 件選択", "Показано: %d · выбрано: %d", "%d affichés · %d sélectionnés"},
		xSkillsFixSelected:      {"Fix (%d)", "修复（%d）", "修復（%d）", "수정 (%d)", "修正（%d）", "Исправить (%d)", "Corriger (%d)"},
		xSkillsDeleteSelected:   {"Delete (%d)", "删除（%d）", "刪除（%d）", "삭제 (%d)", "削除（%d）", "Удалить (%d)", "Supprimer (%d)"},
		xSkillsBatchFixTitle:    {"Fix %d selected skills", "修复所选的 %d 个 skill", "修復所選的 %d 個 skill", "선택한 skill %d개 수정", "選択した %d 個の skill を修正", "Исправить выбранные навыки: %d", "Corriger %d skills sélectionnés"},
		xSkillsBatchFixMsg:      {"Each skill gets its first fix, one at a time:", "每个 skill 采用其首选的修复方式，逐个执行：", "每個 skill 採用其首選的修復方式，逐一執行：", "각 skill에 첫 번째 수정 방법을 하나씩 적용합니다:", "各 skill に最初の修正方法を 1 つずつ適用します：", "К каждому навыку по очереди применяется первый способ исправления:", "Chaque skill reçoit sa première correction, l’un après l’autre :"},
		xSkillsBatchDescNote:    {"Descriptions use the suggested text (the first paragraph or the name); you can edit SKILL.md later.", "描述使用自动生成的建议文字（正文首段或名称），之后可在 SKILL.md 中修改。", "描述使用自動產生的建議文字（正文首段或名稱），之後可在 SKILL.md 中修改。", "설명에는 제안 문구(본문 첫 단락 또는 이름)를 사용하며, 나중에 SKILL.md에서 고칠 수 있습니다.", "説明には提案テキスト（本文の最初の段落または名前）を使います。あとで SKILL.md で編集できます。", "Описание берётся из предложенного текста (первый абзац или имя); позже его можно изменить в SKILL.md.", "Les descriptions reprennent le texte suggéré (premier paragraphe ou nom) ; vous pourrez modifier SKILL.md ensuite."},
		xSkillsBatchSkipFix:     {"%d selected skills have no fix and are skipped.", "%d 个所选 skill 没有可用的修复，将跳过。", "%d 個所選 skill 沒有可用的修復，將略過。", "선택한 skill 중 %d개는 수정 방법이 없어 건너뜁니다.", "選択した skill のうち %d 個は修正方法がないためスキップします。", "Навыков без исправления (будут пропущены): %d.", "%d skills sélectionnés n’ont pas de correction et seront ignorés."},
		xSkillsBatchDeleteTitle: {"Delete %d selected skills", "删除所选的 %d 个 skill", "刪除所選的 %d 個 skill", "선택한 skill %d개 삭제", "選択した %d 個の skill を削除", "Удалить выбранные навыки: %d", "Supprimer %d skills sélectionnés"},
		xSkillsBatchDeleteMsg:   {"They go to the CodexRunway skills trash one by one, where each can be restored.", "这些 skill 会逐个移入 CodexRunway 的 Skill 回收站，可在回收站中分别恢复。", "這些 skill 會逐一移入 CodexRunway 的 Skill 回收站，可在回收站中分別復原。", "하나씩 CodexRunway Skill 휴지통으로 이동하며, 휴지통에서 각각 복원할 수 있습니다.", "1 つずつ CodexRunway の Skill ゴミ箱に移動し、ゴミ箱から個別に復元できます。", "Навыки по одному попадут в корзину навыков CodexRunway, откуда каждый можно восстановить.", "Ils vont un par un dans la corbeille de skills de CodexRunway, d’où chacun peut être restauré."},
		xSkillsBatchSkipDelete:  {"%d selected skills cannot be deleted and are skipped.", "%d 个所选 skill 不能删除，将跳过。", "%d 個所選 skill 無法刪除，將略過。", "선택한 skill 중 %d개는 삭제할 수 없어 건너뜁니다.", "選択した skill のうち %d 個は削除できないためスキップします。", "Навыков, которые нельзя удалить (будут пропущены): %d.", "%d skills sélectionnés ne peuvent pas être supprimés et seront ignorés."},
		xSkillsBatchFixed:       {"Fixed %d items; restart running CLIs to apply", "已修复 %d 项；已在运行的 CLI 重启后生效", "已修復 %d 項；已在執行的 CLI 重新啟動後生效", "%d개 항목을 수정했습니다. 실행 중인 CLI를 다시 시작하면 적용됩니다", "%d 件を修正しました。実行中の CLI を再起動すると反映されます", "Исправлено: %d; перезапустите запущенные CLI", "%d éléments corrigés ; redémarrez les CLI ouverts pour appliquer"},
		xSkillsBatchDeleted:     {"Moved %d items to the trash", "已将 %d 项移入回收站", "已將 %d 項移入回收站", "%d개 항목을 휴지통으로 옮겼습니다", "%d 件をゴミ箱に移動しました", "Перемещено в корзину: %d", "%d éléments mis à la corbeille"},
		xSkillsBatchFailed:      {"Not everything was done: %s", "部分操作未完成：%s", "部分操作未完成：%s", "일부 작업을 완료하지 못했습니다: %s", "一部の操作を完了できませんでした：%s", "Выполнено не всё: %s", "Tout n’a pas pu être fait : %s"},
		xSkillsBatchPartial:     {"%d done, %d failed; %s", "成功 %d 项，失败 %d 项；%s", "成功 %d 項，失敗 %d 項；%s", "%d개 성공, %d개 실패; %s", "成功 %d 件、失敗 %d 件；%s", "успешно: %d, с ошибкой: %d; %s", "%d réussis, %d en échec ; %s"},
	} {
		extraTexts[key] = texts
	}
}
