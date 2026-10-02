import AppKit
import CodexRunwayCore

extension StatusController {
    func populateMenu(_ menu: NSMenu) {
        let l10n = settings.l10n
        menu.removeAllItems()
        menu.addItem(menuItem(l10n.text(.showDetails), action: #selector(showDetailsFromMenu)))
        menu.addItem(menuItem(l10n.text(.openDetailsWindow), action: #selector(openDetailsWindowFromMenu)))
        menu.addItem(menuItem(l10n.text(.openControlPanel), action: #selector(openControlPanelFromMenu)))
        menu.addItem(menuItem(l10n.text(.refresh), action: #selector(refreshFromMenu)))
        menu.addItem(menuItem(l10n.text(.checkForUpdates), action: #selector(checkForUpdatesFromMenu)))
        if model.selectedProvider == .codex {
            menu.addItem(NSMenuItem.separator())
            menu.addItem(menuItem(l10n.text(.repairIndex), action: #selector(repairFromMenu)))
            menu.addItem(menuItem(l10n.text(.codexFolder), action: #selector(openCodexFolder)))
        }
        menu.addItem(NSMenuItem.separator())
        menu.addItem(menuItem(l10n.text(.quit), action: #selector(quit)))
    }

    private func menuItem(_ title: String, action: Selector) -> NSMenuItem {
        let item = NSMenuItem(title: title, action: action, keyEquivalent: "")
        item.target = self
        return item
    }
}
