---
layout: splash
permalink: /
hidden: true
header:
  overlay_color: "#1f4ac2"
  overlay_image: /assets/images/home-bg.jpg
  actions:
    - label: "<i class='fas fa-download'></i> Install now"
      url: "https://github.com/gopaddle-io/configurator/raw/main/helm/configurator-0.1.0.tgz"
excerpt: >
  Never let a ConfigMap update break your Kubernetes deployment<br />
  <small><a href="https://github.com/gopaddle-io/configurator/releases/tag/v0.0.1">Latest release v.0.0.1</a></small>
feature_row1:
  - image_path: /assets/images/github.png
    alt: "Github repo"
    title: "Github project repo"
    excerpt: "Visit the project repository to clone, fork, customize the project…"
    url: "https://github.com/gopaddle-io/configurator"
    btn_class: "btn--primary"
    btn_label: "Repo Link"
  - image_path: /assets/images/discord.png
    alt: "Discord"
    title: "Discord Community"
    excerpt: "If you want to contribute to the project but have no idea where to start, join the discord server where a helping hand is always welcome."
    url: "https://discord.com/invite/dr24Z4BmP8"
    btn_class: "btn--primary"
    btn_label: "Join Server"
  - image_path: /assets/images/Introduction_to_configurator.png
    alt: "Introducing Configurator"
    title: "Introduction to Configurator"
    excerpt: "Watch this introductory video to get a grasp of the porblem with ConfigMaps and the strategy we're using to solve it."
    url: "/An-Introduction-to-Configurator/"
    btn_class: "btn--primary"
    btn_label: "Video link"

#   - image_path: /assets/images/apache_logo.png
#     alt: "100% free"
#     title: "100% free"
#     excerpt: "Free to use however you want under the Apache License. Clone it, fork it, customize it..."
#     url: "/license/"
#     btn_class: "btn--primary"
#     btn_label: "License"      
---

<span style="font-size:0.5em"><a href="https://www.freepik.com/vectors/background">Background vector created by freepik - www.freepik.com</a></span>

{% include feature_row id="feature_row1"%}

## Thanks to all contributors for their effort

<script type="module">
import { Octokit } from "https://cdn.skypack.dev/@octokit/core";

const octokit = new Octokit();

await octokit.request('GET /repos/gopaddle-io/configurator/stats/contributors', {
  owner: 'gopaddle-io',
  repo: 'configurator'

}).then((resp) => {
  resp.data.forEach((r) => {
    console.log(r.author.login);
  })
  });</script>
