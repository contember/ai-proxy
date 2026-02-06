class Tudy < Formula
  desc "AI-powered local development proxy"
  homepage "https://github.com/contember/tudy"
  version "0.4.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/contember/tudy/releases/download/v#{version}/caddy-darwin-arm64.tar.gz"
      sha256 "1c609013a70007df8d6dcea4eaa4d3ab8c4062bf65d0ccce418f1bd8da5a7f9b"
    end
    on_intel do
      url "https://github.com/contember/tudy/releases/download/v#{version}/caddy-darwin-amd64.tar.gz"
      sha256 "e3b66ef535b785b9310a3ea6af49ecf41fc4d9b968e8b75332af21da8a381e6e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/contember/tudy/releases/download/v#{version}/caddy-linux-arm64.tar.gz"
      sha256 "8d4c3192e1ac0371229205960bd43266292d80a0275b68a92ec56b9a2dae3b8d"
    end
    on_intel do
      url "https://github.com/contember/tudy/releases/download/v#{version}/caddy-linux-amd64.tar.gz"
      sha256 "9c240b1f10d9e0153726a11dab4b44a3bfd979310021872c15d234d833b6f8e5"
    end
  end

  def install
    bin.install "caddy" => "tudy-bin"
    bin.install "cli" => "tudy"
    (etc/"tudy").mkpath
    (etc/"tudy").install "Caddyfile" unless (etc/"tudy/Caddyfile").exist?

    # Install menubar app (macOS only)
    if File.exist?("menubar") && OS.mac?
      # Create app bundle
      app_name = "Tudy.app"
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
          <string>com.contember.tudy</string>
          <key>CFBundleName</key>
          <string>Tudy</string>
          <key>CFBundleDisplayName</key>
          <string>Tudy</string>
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
    run [opt_bin/"tudy", "run", "--config", etc/"tudy/Caddyfile"]
    keep_alive true
    log_path var/"log/tudy.log"
    error_log_path var/"log/tudy.error.log"
    environment_variables XDG_DATA_HOME: HOMEBREW_PREFIX/"share", XDG_CONFIG_HOME: HOMEBREW_PREFIX/"etc"
  end

  def post_install
    (var/"lib/tudy").mkpath
    # Create Caddy data directory for PKI certificates
    (share/"caddy").mkpath
    # Create example env file if it doesn't exist
    env_file = etc/"tudy/env"
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
      Run the interactive setup to configure API key, trust certificate, and start:
        tudy setup

      Or configure manually:
        echo "LLM_API_KEY=sk-your-key" >> #{etc}/tudy/env
        sudo brew services start tudy

      Logs: #{var}/log/tudy.log
    EOS

    if OS.mac?
      s += <<~EOS

        Menu bar app installed at:
          #{opt_prefix}/Tudy.app

        To add to Login Items, drag the app to System Settings > General > Login Items
      EOS
    end

    s
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/tudy version")
  end
end
