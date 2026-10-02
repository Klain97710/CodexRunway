import AppKit
import CodexRunwayCore
import SwiftUI

/// Renders the rate-limit-reset section with fixture data for design checks.
enum RateLimitResetTodayMockRender {
    struct QACase {
        var name: String
        var kind: RateLimitResetTodaySnapshot.DevMockKind
        var language: ResolvedLanguage
        var width: CGFloat
        var colorScheme: ColorScheme
        var confidence: Double? = nil
    }

    static let qaCases: [QACase] = [
        QACase(name: "completed-en-light-280", kind: .completed, language: .english, width: 280, colorScheme: .light),
        QACase(
            name: "scheduled-zh-hans-dark-358",
            kind: .explicitScheduled,
            language: .simplifiedChinese,
            width: 358,
            colorScheme: .dark,
            confidence: 0.88),
        QACase(name: "inferred-zh-hant-light-400", kind: .inferredScheduled, language: .traditionalChinese, width: 400, colorScheme: .light),
        QACase(
            name: "grace-ja-dark-280",
            kind: .grace,
            language: .japanese,
            width: 280,
            colorScheme: .dark,
            confidence: 0.92),
        QACase(name: "expired-ko-light-358", kind: .expired, language: .korean, width: 358, colorScheme: .light),
        QACase(name: "unavailable-fr-dark-400", kind: .unavailable, language: .french, width: 400, colorScheme: .dark),
        QACase(name: "no-ru-light-280", kind: .no, language: .russian, width: 280, colorScheme: .light),
        QACase(name: "unavailable-ja-dark-280", kind: .unavailable, language: .japanese, width: 280, colorScheme: .dark),
        QACase(name: "unavailable-fr-light-280", kind: .unavailable, language: .french, width: 280, colorScheme: .light),
    ]

    @MainActor
    static func render(
        kind: RateLimitResetTodaySnapshot.DevMockKind,
        language: ResolvedLanguage = .simplifiedChinese,
        width: CGFloat = 358,
        resetType: RateLimitResetType = .global,
        colorScheme: ColorScheme = .light,
        confidence: Double? = nil) throws -> Data
    {
        let host = NSHostingView(rootView: root(
            kind: kind,
            language: language,
            width: width,
            resetType: resetType,
            colorScheme: colorScheme,
            confidence: confidence))
        layout(host, width: width)

        guard let rep = host.bitmapImageRepForCachingDisplay(in: host.bounds) else {
            throw CocoaError(.fileWriteUnknown)
        }
        host.cacheDisplay(in: host.bounds, to: rep)
        guard let data = rep.representation(using: .png, properties: [:]) else {
            throw CocoaError(.fileWriteUnknown)
        }
        return data
    }

    @MainActor
    static func logicalSize(
        kind: RateLimitResetTodaySnapshot.DevMockKind,
        language: ResolvedLanguage,
        width: CGFloat,
        resetType: RateLimitResetType,
        colorScheme: ColorScheme,
        confidence: Double? = nil) -> CGSize
    {
        let host = NSHostingView(rootView: root(
            kind: kind,
            language: language,
            width: width,
            resetType: resetType,
            colorScheme: colorScheme,
            confidence: confidence))
        layout(host, width: width)
        return host.frame.size
    }

    @MainActor
    private static func root(
        kind: RateLimitResetTodaySnapshot.DevMockKind,
        language: ResolvedLanguage,
        width: CGFloat,
        resetType: RateLimitResetType,
        colorScheme: ColorScheme,
        confidence: Double?) -> some View
    {
        var snapshot = RateLimitResetTodaySnapshot.devMock(kind: kind)
        if !snapshot.events.isEmpty {
            snapshot.events[0].resetType = resetType
        }
        if let confidence {
            for index in snapshot.events.indices {
                snapshot.events[index].confidence = confidence
            }
            snapshot.resetTimeline?.nextSchedule?.confidence = confidence
        }
        return RateLimitResetTodayView(
            snapshot: snapshot,
            l10n: L10n(language: language),
            isRefreshing: false,
            onRefresh: {},
            onOpenSource: {},
            onOpenEvidence: { _ in })
            .padding(16)
            .frame(width: width)
            .background(Color(nsColor: .windowBackgroundColor))
            .environment(\.colorScheme, colorScheme)
    }

    @MainActor
    private static func layout<Content: View>(_ host: NSHostingView<Content>, width: CGFloat) {
        host.frame = NSRect(x: 0, y: 0, width: width, height: 10)
        for _ in 0..<3 {
            host.layoutSubtreeIfNeeded()
            RunLoop.main.run(until: Date().addingTimeInterval(0.01))
        }
        let fittingHeight = host.fittingSize.height
        let height = fittingHeight < 80 ? 180 : fittingHeight
        host.frame = NSRect(x: 0, y: 0, width: width, height: height)
        host.layoutSubtreeIfNeeded()
    }

    @MainActor
    static func write(
        kind: RateLimitResetTodaySnapshot.DevMockKind,
        language: ResolvedLanguage = .simplifiedChinese,
        to path: String) throws
    {
        let data = try render(kind: kind, language: language)
        let url = URL(fileURLWithPath: path)
        try FileManager.default.createDirectory(
            at: url.deletingLastPathComponent(),
            withIntermediateDirectories: true)
        try data.write(to: url)
        print("wrote \(path) (\(data.count) bytes)")
    }

    @MainActor
    static func writeQAMatrix(to directory: String) throws {
        let root = URL(fileURLWithPath: directory, isDirectory: true)
        try FileManager.default.createDirectory(
            at: root,
            withIntermediateDirectories: true)
        for item in qaCases {
            let data = try render(
                kind: item.kind,
                language: item.language,
                width: item.width,
                colorScheme: item.colorScheme,
                confidence: item.confidence)
            let url = root.appendingPathComponent("\(item.name).png")
            try data.write(to: url)
            print("wrote \(url.path) (\(data.count) bytes)")
        }
    }
}
