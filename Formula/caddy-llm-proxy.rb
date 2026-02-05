class CaddyLlmProxy < Formula
  desc "Caddy with LLM-based dynamic routing plugin"
  homepage "https://github.com/contember/ai-proxy"
  version "0.2.5"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-darwin-arm64.tar.gz"
      sha256 "ea924160755575868682b54548b6967787b464ef3b1dceeab2c1579165604c1d"
    end
    on_intel do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-darwin-amd64.tar.gz"
      sha256 "e3b66ef535b785b9310a3ea6af49ecf41fc4d9b968e8b75332af21da8a381e6e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-linux-arm64.tar.gz"
      sha256 "0785c8ff14ff5d7df9234ea0ecfd4a1eafbb1fc37ff83682f85b2267ec519f29"
    end
    on_intel do
      url "https://github.com/contember/ai-proxy/releases/download/v#{version}/caddy-linux-amd64.tar.gz"
      sha256 "9c240b1f10d9e0153726a11dab4b44a3bfd979310021872c15d234d833b6f8e5"
    end
  end

  def install
    bin.install "caddy" => "caddy-llm-proxy-bin"
    (etc/"caddy-llm-proxy").mkpath
    (etc/"caddy-llm-proxy").install "Caddyfile" unless (etc/"caddy-llm-proxy/Caddyfile").exist?

    # Create wrapper script that loads env file
    (bin/"caddy-llm-proxy").write <<~EOS
      #!/bin/bash
      set -a
      [ -f "#{etc}/caddy-llm-proxy/env" ] && source "#{etc}/caddy-llm-proxy/env"
      set +a
      export CADDY_DATA_DIR="${CADDY_DATA_DIR:-#{var}/lib/caddy-llm-proxy}"
      exec "#{opt_bin}/caddy-llm-proxy-bin" "$@"
    EOS
    (bin/"caddy-llm-proxy").chmod 0755

    # Install menubar app (macOS only)
    if File.exist?("menubar") && OS.mac?
      # Create app bundle
      app_name = "Caddy LLM Proxy.app"
      app_path = prefix/app_name
      (app_path/"Contents/MacOS").mkpath
      (app_path/"Contents/Resources").mkpath

      cp "menubar", app_path/"Contents/MacOS/menubar"
      chmod 0755, app_path/"Contents/MacOS/menubar"

      # Create Info.plist
      (app_path/"Contents/Info.plist").write <<~XML
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
          <key>CFBundleExecutable</key>
          <string>menubar</string>
          <key>CFBundleIdentifier</key>
          <string>com.contember.caddy-llm-proxy</string>
          <key>CFBundleName</key>
          <string>Caddy LLM Proxy</string>
          <key>CFBundleDisplayName</key>
          <string>Caddy LLM Proxy</string>
          <key>CFBundleVersion</key>
          <string>#{version}</string>
          <key>CFBundleShortVersionString</key>
          <string>#{version}</string>
          <key>CFBundlePackageType</key>
          <string>APPL</string>
          <key>LSUIElement</key>
          <true/>
          <key>LSMinimumSystemVersion</key>
          <string>10.13</string>
          <key>NSHighResolutionCapable</key>
          <true/>
        </dict>
        </plist>
      XML

      # Link to Applications via prefix
      (prefix/"Applications").mkpath
      ln_sf app_path, prefix/"Applications/#{app_name}"
    end
  end

  service do
    run [opt_bin/"caddy-llm-proxy", "run", "--config", etc/"caddy-llm-proxy/Caddyfile"]
    keep_alive true
    log_path var/"log/caddy-llm-proxy.log"
    error_log_path var/"log/caddy-llm-proxy.error.log"
    environment_variables XDG_DATA_HOME: HOMEBREW_PREFIX/"share", XDG_CONFIG_HOME: HOMEBREW_PREFIX/"etc"
  end

  def post_install
    (var/"lib/caddy-llm-proxy").mkpath
    # Create Caddy data directory for PKI certificates
    (share/"caddy").mkpath
    # Create example env file if it doesn't exist
    env_file = etc/"caddy-llm-proxy/env"
    unless env_file.exist?
      env_file.write <<~EOS
        # Add your API key here:
        # LLM_API_KEY=sk-your-key-here
        #
        # Optional settings:
        # LLM_API_URL=https://openrouter.ai/api/v1/chat/completions
        # MODEL=anthropic/claude-haiku-4.5
      EOS
    end
  end

  def caveats
    s = <<~EOS
      Add your API key to #{etc}/caddy-llm-proxy/env:
        echo "LLM_API_KEY=sk-your-key" >> #{etc}/caddy-llm-proxy/env

      Start the service:
        sudo brew services start caddy-llm-proxy

      Logs: #{var}/log/caddy-llm-proxy.log
    EOS

    if OS.mac?
      s += <<~EOS

        Menu bar app installed at:
          #{opt_prefix}/Caddy LLM Proxy.app

        To add to Login Items, drag the app to System Settings > General > Login Items
      EOS
    end

    s
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/caddy-llm-proxy version")
  end
end
