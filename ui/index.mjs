var map = L.map("map").setView([54.78646834434753, 9.438009324512887], 13);

L.tileLayer("https://tile.openstreetmap.org/{z}/{x}/{y}.png", {
  maxZoom: 19,
  attribution:
    '&copy; <a href="http://www.openstreetmap.org/copyright">OpenStreetMap</a>',
}).addTo(map);

const startPoint = [54.76800589462181, 9.432455715803746];
const endPoint = [54.76950271898897, 9.436204955830489];
let wateringPoints = [
  [54.768557846288275, 9.434116547866934],
  [54.80539072138063, 9.449963237272696],
];

const markerHtmlStyles = (color) => `
  background-color: ${color};
  width: 2rem;
  height: 2rem;
  position: absolute;
  border-radius: 3rem;
  left: 0.25rem;
  top: 0.25rem;
  border: 1px solid white;
  display: flex;
  align-items: center;
  justify-content: center;
`;

const treeIcon = L.divIcon({
  className: "treeMarkerIcon",
  iconAnchor: [0, 24],
  labelAnchor: [-6, 0],
  popupAnchor: [0, -36],
  html: `<span style="${markerHtmlStyles("green")}" />`,
});

const startPointIcon = L.divIcon({
  className: "startMarkerIcon",
  iconAnchor: [0, 24],
  labelAnchor: [-6, 0],
  popupAnchor: [0, -36],
  html: `<span style="${markerHtmlStyles("orange")}" />`,
});

const endPointIcon = L.divIcon({
  className: "endMarkerIcon",
  iconAnchor: [0, 24],
  labelAnchor: [-6, 0],
  popupAnchor: [0, -36],
  html: `<span style="${markerHtmlStyles("red")}" />`,
});

L.marker(startPoint, { icon: startPointIcon })
  .bindPopup("startPoint")
  .addTo(map);

L.marker(endPoint, { icon: endPointIcon }).bindPopup("endPoint").addTo(map);

wateringPoints.map((w, i) =>
  L.marker(w)
    .bindPopup(`wateringPoint ${i + 1}`)
    .addTo(map),
);

document.addEventListener("DOMContentLoaded", () => {
  document.getElementById("startpoint-btn").addEventListener("click", () => {
    map.setView(startPoint, 17);
  });

  document.getElementById("endpoint-btn").addEventListener("click", () => {
    map.setView(endPoint, 17);
  });

  document.getElementById("wateringlist").innerHTML =
    "<b>watering points: </b>" + wateringPoints.length;

  map.on("click", (e) => {
    wateringPoints.push([e.latlng.lat, e.latlng.lng]);
    document.getElementById("wateringlist").innerHTML =
      "<b>watering points: </b>" + wateringPoints.length;

    L.marker([e.latlng.lat, e.latlng.lng]).addTo(map);
  });
});

let treeList = [];
fetch("/trees.json")
  .then((res) => res.json())
  .then((data) =>
    data.map((t) =>
      L.marker([t.lat, t.lng], { icon: treeIcon })
        .addTo(map)
        .on("click", () => {
          treeList.includes(t)
            ? (treeList = treeList.filter((v) => v.id !== t.id))
            : treeList.push(t);
          document.getElementById("treelist").innerHTML =
            "<b>selected trees:</b> " + treeList.map((t) => t.id).join(", ");
        }),
    ),
  );

let shownNearLines = [];
document.getElementById("nearest-btn").addEventListener("click", () => {
  fetch("/v1/nearest", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      trees: treeList.map((t) => ({ lat: t.lat, lng: t.lng })),
      wp: wateringPoints.map(([lat, lng]) => ({ lat, lng })),
    }),
  })
    .then((res) => res.json())
    .then(displayLines);
});

document.getElementById("nearest-btn").addEventListener("click", () => {
  fetch("/v1/nearest", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      trees: treeList.map((t) => ({ lat: t.lat, lng: t.lng })),
      wp: wateringPoints.map(([lat, lng]) => ({ lat, lng })),
    }),
  })
    .then((res) => res.json())
    .then(displayLines);
});

document.getElementById("nearest-v2-btn").addEventListener("click", () => {
  fetch("/v2/nearest", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      trees: treeList.map((t) => ({ lat: t.lat, lng: t.lng })),
      wp: wateringPoints.map(([lat, lng]) => ({ lat, lng })),
    }),
  })
    .then((res) => res.json())
    .then((data) => {
      const flatData = data.sources_to_targets.flat();

      const mapped = data.targets
        .map((_, i) => {
          const filteredData = flatData.filter((f) => f.to_index === i);

          return filteredData.reduce(
            (acc, f) => (f.distance < acc.distance ? f : acc),
            filteredData[0],
          );
        })
        .map((v) => ({
          wp: {
            lat: data.sources[v.from_index].lat,
            lng: data.sources[v.from_index].lon,
          },
          tree: {
            lat: data.targets[v.to_index].lat,
            lng: data.targets[v.to_index].lon,
          },
        }));

      displayLines(mapped);
    });
});

const displayLines = (data) => {
  shownNearLines.forEach((l) => map.removeLayer(l));
  data.forEach((d) => {
    const line = L.polyline(
      [
        [d.tree.lat, d.tree.lng],
        [d.wp.lat, d.wp.lng],
      ],
      {
        color: "blue",
        weight: 4,
        smoothFactor: 1,
      },
    ).addTo(map);

    shownNearLines.push(line);
  });
};
