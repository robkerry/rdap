class Rdap < Formula
  desc "Direct IANA-bootstrap RDAP command-line client"
  homepage "https://github.com/robkerry/rdap"
  version "1.0.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/robkerry/rdap/releases/download/v1.0.1/rdap_1.0.1_darwin_arm64.tar.gz"
      sha256 "9fcb0813af773866a6f2456b1972eb14a9764ed79abc885cbfb5e06e23a36fbe"
    end
    on_intel do
      url "https://github.com/robkerry/rdap/releases/download/v1.0.1/rdap_1.0.1_darwin_amd64.tar.gz"
      sha256 "61bf886b4cf3a9ff802e2ae7362257733f2e53295aef757f921707b61cb1168c"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/robkerry/rdap/releases/download/v1.0.1/rdap_1.0.1_linux_arm64.tar.gz"
      sha256 "b3075ec90139ccce32ee9177a32a33e864d098ef3e0815a9c937933b354edc8c"
    end
    on_intel do
      url "https://github.com/robkerry/rdap/releases/download/v1.0.1/rdap_1.0.1_linux_amd64.tar.gz"
      sha256 "5023776ba4de21fe8fb0d13be293a3d8a80a6754bde1c0bc82aa266130732f0a"
    end
  end

  def install
    bin.install "rdap"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/rdap --version")
  end
end
