async function nextEpisode(id) {
  await fetch(`/update?id=${id}`, { method: "POST" });
  location.reload();
}