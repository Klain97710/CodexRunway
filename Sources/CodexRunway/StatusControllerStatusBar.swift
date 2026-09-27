import AppKit
import CodexRunwayCore

@MainActor
extension StatusController {
    func installStatusBarView() {
        guard let button = statusItem.button else { return }
        button.title = ""
        statusBarView.translatesAutoresizingMaskIntoConstraints = false
        button.addSubview(statusBarView)
        NSLayoutConstraint.activate([
            statusBarView.leadingAnchor.constraint(equalTo: button.leadingAnchor),
            statusBarView.trailingAnchor.constraint(equalTo: button.trailingAnchor),
            statusBarView.topAnchor.constraint(equalTo: button.topAnchor),
            statusBarView.bottomAnchor.constraint(equalTo: button.bottomAnchor),
        ])
        updateStatusBarView()
    }

    func updateStatusBarView() {
        let state = StatusBarContentState(
            configuration: StatusBarContentState.Configuration(
                preferences: settings.preferences,
                language: settings.l10n.language),
            content: StatusBarContentState.Content(
                text: model.selectedStatusText,
                meters: model.selectedQuotaMeters,
                displayMinute: Int(Date().timeIntervalSince1970 / 60)))
        guard Self.updateStatusBarContent(state, statusItem: statusItem, contentView: statusBarView) else { return }
        let quotaDetails = model.selectedQuotaMeters
            .map { "\($0.title): \($0.remainingPercent)%" }
            .joined(separator: " · ")
        statusItem.button?.toolTip = quotaDetails.isEmpty
            ? "CodexRunway · \(model.selectedStatusText)"
            : "CodexRunway · \(model.selectedStatusText)\n\(quotaDetails)"
    }

    @discardableResult
    static func updateStatusBarContent(
        _ state: StatusBarContentState,
        statusItem: NSStatusItem,
        contentView: StatusBarContentView) -> Bool
    {
        guard contentView.update(state) else { return false }
        let layout = StatusBarContentLayout(state: state)
        let usesNativeText = state.configuration.style == .text
        contentView.isHidden = usesNativeText
        // Let AppKit render the title's color and inactive-menu-bar appearance.
        statusItem.button?.font = layout.textFont
        statusItem.button?.title = usesNativeText ? layout.textCaptions.joined(separator: "  ") : ""
        statusItem.length = usesNativeText ? NSStatusItem.variableLength : contentView.preferredWidth
        return true
    }
}
