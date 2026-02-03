class CaddyLlmProxy < Formula
  desc "Caddy with LLM-based dynamic routing plugin"
  homepage "https://github.com/contember/ai-proxy"
  version "0.0.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-darwin-arm64.tar.gz"
      sha256 "SHA256_DARWIN_ARM64"
    end
    on_intel do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-darwin-amd64.tar.gz"
      sha256 "SHA256_DARWIN_AMD64"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-linux-arm64.tar.gz"
      sha256 "SHA256_LINUX_ARM64"
    end
    on_intel do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-linux-amd64.tar.gz"
      sha256 "SHA256_LINUX_AMD64"
    end
  end

  def install
    bin.install "caddy" => "caddy-llm-proxy"
  end

  service do
    run [opt_bin/"caddy-llm-proxy", "run", "--config", etc/"Caddyfile"]
    keep_alive true
    log_path var/"log/caddy-llm-proxy.log"
    error_log_path var/"log/caddy-llm-proxy.error.log"
  end

  def caveats
    <<~EOS
      To use the LLM proxy, set the following environment variables:
        export LLM_API_KEY="your-api-key"

      Create a Caddyfile at #{etc}/Caddyfile with your configuration.

      To start the service:
        brew services start caddy-llm-proxy
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/caddy-llm-proxy version")
  end
end
