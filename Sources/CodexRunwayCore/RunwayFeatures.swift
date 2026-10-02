/// Shared runtime capabilities. Retained provider code can be exercised explicitly in tests.
public struct RunwayFeatures: Equatable, Sendable {
    public let grokEnabled: Bool

    public init(grokEnabled: Bool) {
        self.grokEnabled = grokEnabled
    }

    public static let production = Self(grokEnabled: false)
    public static let allProviders = Self(grokEnabled: true)

    public func provider(_ value: RunwayProvider) -> RunwayProvider {
        grokEnabled ? value : .codex
    }

    public func widgetScope(_ value: RunwayWidgetProviderScope) -> RunwayWidgetProviderScope {
        grokEnabled ? value : .codex
    }

    public func preferences(_ value: RunwayPreferences) -> RunwayPreferences {
        guard !grokEnabled else { return value }
        var result = value
        result.selectedProvider = .codex
        result.statusBarProviderScope = .selected
        return result
    }
}
