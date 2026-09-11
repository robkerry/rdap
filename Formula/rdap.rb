class Rdap < Formula
  desc "Direct IANA-bootstrap RDAP command-line client"
  homepage "https://github.com/robkerry/rdap"
  version "1.0.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/robkerry/rdap/releases/download/v1.0.0/rdap_1.0.0_darwin_arm64.tar.gz"
      sha256 "fdfdd0d89231c0e5ba6bd9d8cd78fa80026d745efc0ba90b58ab6a6e0cea0e21"
    end
    on_intel do
      url "https://github.com/robkerry/rdap/releases/download/v1.0.0/rdap_1.0.0_darwin_amd64.tar.gz"
      sha256 "8ec5fd8550d8892254c7d7ce36a5db39a2f88af24b9516bbb5abae19c6c6a4e5"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/robkerry/rdap/releases/download/v1.0.0/rdap_1.0.0_linux_arm64.tar.gz"
      sha256 "a03fb112266b8d7953c5ac4af01a6497bbad5c84217ed85833e22cbfc0d6bd68"
    end
    on_intel do
      url "https://github.com/robkerry/rdap/releases/download/v1.0.0/rdap_1.0.0_linux_amd64.tar.gz"
      sha256 "242ad04af26a6ade331052218646cb53d3e5067ccc7c7cb67509ded399187b1a"
    end
  end

  def install
    bin.install "rdap"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/rdap --version")
  end
end
