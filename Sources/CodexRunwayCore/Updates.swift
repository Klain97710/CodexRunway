import Foundation

public enum UpdateInstallReadiness: Equatable, Sendable {
    case ready
    case developmentMode
    case signingKeyMissing
}

public struct UpdateInstallEnvironment: Sendable {
    public let bundlePathExtension: String
    public let sparklePublicKey: String?
    public let hasUpdater: Bool
    public let updatesEnabled: Bool

    public init(bundlePathExtension: String, sparklePublicKey: String?, hasUpdater: Bool, updatesEnabled: Bool = true) {
        self.bundlePathExtension = bundlePathExtension
        self.sparklePublicKey = sparklePublicKey
        self.hasUpdater = hasUpdater
        self.updatesEnabled = updatesEnabled
    }

    public var readiness: UpdateInstallReadiness {
        guard updatesEnabled, bundlePathExtension.lowercased() == "app" else { return .developmentMode }
        guard Self.hasValidSparklePublicKey(sparklePublicKey), hasUpdater else { return .signingKeyMissing }
        return .ready
    }

    public func shouldCheckForUpdatesOnLaunch(automaticallyChecksForUpdates: Bool) -> Bool {
        automaticallyChecksForUpdates && readiness == .ready
    }

    public static func hasValidSparklePublicKey(_ key: String?) -> Bool {
        guard let key, let data = Data(base64Encoded: key) else { return false }
        return data.count == 32
    }
}
