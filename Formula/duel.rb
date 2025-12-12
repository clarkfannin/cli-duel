class Duel < Formula
  desc "CLI 1v1 dueling game"
  homepage "https://github.com/clarkfannin/cli-duel"
  url "https://github.com/clarkfannin/cli-duel/archive/refs/tags/v0.2.2.tar.gz"
  sha256 "a84201a064f29a23ef9b68a6e5e43a51d06dcaf99a3b03c33e8b53d1ee2dd705"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-s -w"), "-o", bin/"duel", "."
  end

  test do
    assert_match "duel", shell_output("#{bin}/duel help 2>&1", 0)
  end
end
