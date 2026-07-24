n="sillyGirl"
s="/usr/local/$n"
a=arm64
repo="smallfawn/sillyGirl"
proxy="${GITHUB_PROXY:-https://gh-proxy.org}"
if [[ $(uname -a | grep "x86_64") != "" ]]; then 
    a=amd64
fi ;
if [ ! -d $s ]; then 
    mkdir $s
fi ;
cd $s;
rm -rf $n;
v=`curl -fsSL --max-time 10 "$proxy/https://raw.githubusercontent.com/$repo/refs/heads/main/VERSION" | head -n 1 | tr -d "[:space:]"`
v="${v#v}"
if [ -z "$v" ]; then
    echo "版本获取失败，请检查网络或设置 GITHUB_PROXY。"
    exit
fi
tag="v$v"
asset="sillyGirl_${tag}_linux_${a}.tar.gz"
d="$proxy/https://github.com/$repo/releases/download/${tag}/${asset}"
echo "检测到版本 $tag"
echo "正在从 $d 下载..."
curl -fL -o "$asset" "$d" && tar -xzf "$asset" && mv "sillyGirl_${tag}_linux_${a}" "$n" && chmod 755 "$n"
echo "傻妞已安装到 $s"
echo "请手动运行 $s/$n 带 -t 进入交互模式"
