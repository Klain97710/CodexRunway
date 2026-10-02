import CryptoKit
import Foundation

guard CommandLine.arguments.count == 2 else {
    fatalError("Usage: swift Scripts/generate-update-keys.swift /absolute/private/backup/directory")
}
let directory = URL(fileURLWithPath: CommandLine.arguments[1], isDirectory: true)
let manager = FileManager.default
try manager.createDirectory(at: directory, withIntermediateDirectories: true, attributes: [.posixPermissions: 0o700])
try manager.setAttributes([.posixPermissions: 0o700], ofItemAtPath: directory.path)
let privateURL = directory.appendingPathComponent("sparkle-private.key")
let publicURL = directory.appendingPathComponent("sparkle-public.key")
guard !manager.fileExists(atPath: privateURL.path), !manager.fileExists(atPath: publicURL.path) else {
    fatalError("Key files already exist; preserve the existing release identity.")
}
let key = Curve25519.Signing.PrivateKey()
for (url, data) in [(privateURL, key.rawRepresentation), (publicURL, key.publicKey.rawRepresentation)] {
    guard manager.createFile(atPath: url.path, contents: Data(data.base64EncodedString().utf8),
                             attributes: [.posixPermissions: 0o600]) else {
        fatalError("Could not save release key.")
    }
}
print("Release keys created. Keep this directory outside the repository and back it up securely.")
