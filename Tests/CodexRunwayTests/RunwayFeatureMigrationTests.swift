import Foundation
import Testing
@testable import CodexRunway
@testable import CodexRunwayCore

@Suite("Replacement edition preferences")
@MainActor
struct RunwayFeatureMigrationTests {
    @Test("self-check skips Grok inspection in production")
    func selfCheckSkipsGrok() async {
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        await SelfCheck.run(
            store: CodexAuthStore(authURL: root.appendingPathComponent("auth.json")),
            sessionRepair: SessionRepairService(codexHome: root),
            inspectGrok: { Issue.record("Production self-check inspected Grok") })
    }

    @Test("legacy provider choices migrate without resetting unrelated preferences or proxy")
    func preservesExistingPreferences() throws {
        let suite = "PersonalMigration-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let store = PreferencesStore(defaults: defaults)
        var old = RunwayPreferences(selectedProvider: .grok, language: .french,
            appearance: .dark, statusBarProviderScope: .both,
            refreshIntervalSeconds: 600, showsRateLimitResetToday: false,
            automaticallyChecksForUpdates: false, launchAtLoginEnabled: false,
            launchAtLoginInitialized: true)
        store.save(old)
        let proxyStore = NetworkProxyStore(defaults: defaults)
        let proxy = NetworkProxyConfiguration(mode: .http, host: "localhost", port: 8080)
        try proxyStore.save(proxy)

        let settings = RunwaySettings(store: store, networkProxyStore: proxyStore)
        old.selectedProvider = .codex
        old.statusBarProviderScope = .selected
        #expect(settings.preferences == old)
        #expect(store.load() == old)
        #expect(try proxyStore.load() == proxy)
        settings.updateSelectedProvider(.grok)
        settings.updateStatusBarProviderScope(.both)
        #expect(settings.preferences == old)
        #expect(RunwaySettings(store: store).preferences == old)
    }

    @Test("all legacy widget and deep-link provider values resolve to Codex")
    func savedWidgetChoicesRemainReadable() throws {
        for scope in RunwayWidgetProviderScope.allCases {
            let decoded = try JSONDecoder().decode(RunwayWidgetProviderScope.self,
                from: JSONEncoder().encode(scope))
            let link = try #require(RunwayWidgetDeepLink(url:
                RunwayWidgetDeepLink(provider: decoded, section: .quota).url))
            #expect(RunwayFeatures.production.widgetScope(link.provider) == .codex)
            #expect(RunwayFeatures.allProviders.widgetScope(link.provider) == scope)
        }
    }
}
