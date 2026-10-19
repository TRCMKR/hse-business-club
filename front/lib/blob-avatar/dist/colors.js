function hslToHex(h, s, l) {
  const sat = s / 100;
  const light = l / 100;
  const a = sat * Math.min(light, 1 - light);
  const channel = (n) => {
    const k = (n + h / 30) % 12;
    const c = light - a * Math.max(-1, Math.min(k - 3, 9 - k, 1));
    return Math.round(c * 255).toString(16).padStart(2, "0");
  };
  return `#${channel(0)}${channel(8)}${channel(4)}`;
}
function palette(hue, tone) {
  const h = (hue % 360 + 360) % 360;
  if (tone === "light") {
    return {
      background: hslToHex(h, 80, 91),
      body: hslToHex(h, 72, 67),
      shade: hslToHex(h, 55, 45),
      ink: hslToHex(h, 45, 15),
      sclera: "#fff",
      cheek: hslToHex(352, 90, 72),
      tongue: "#f2647f"
    };
  }
  return {
    background: hslToHex(h, 38, 27),
    body: hslToHex(h, 58, 48),
    shade: hslToHex(h, 55, 30),
    ink: hslToHex(h, 50, 9),
    sclera: "#fff",
    cheek: hslToHex(352, 85, 68),
    tongue: "#f2647f"
  };
}
export {
  palette
};
