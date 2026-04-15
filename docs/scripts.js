(function () {
  "use strict";

  // ─── Scroll Reveal (IntersectionObserver) ───

  var reveals = document.querySelectorAll(".reveal");
  if ("IntersectionObserver" in window && reveals.length) {
    var observer = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (entry.isIntersecting) {
            entry.target.classList.add("visible");
            observer.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.12, rootMargin: "0px 0px -40px 0px" }
    );
    reveals.forEach(function (el) {
      observer.observe(el);
    });
  } else {
    reveals.forEach(function (el) {
      el.classList.add("visible");
    });
  }

  // ─── Active Nav Tracking ───

  var navLinks = document.querySelectorAll("nav a[href^='#']");
  var sections = [];
  navLinks.forEach(function (link) {
    var id = link.getAttribute("href").slice(1);
    var section = document.getElementById(id);
    if (section) sections.push({ el: section, link: link });
  });

  function updateActiveNav() {
    var scrollY = window.scrollY + 100;
    var current = null;
    for (var i = sections.length - 1; i >= 0; i--) {
      if (sections[i].el.offsetTop <= scrollY) {
        current = sections[i];
        break;
      }
    }
    navLinks.forEach(function (l) { l.classList.remove("active"); });
    if (current) current.link.classList.add("active");
  }

  if (sections.length) {
    updateActiveNav();
    window.addEventListener("scroll", updateActiveNav, { passive: true });
  }

  // ─── Header Shadow on Scroll ───

  var header = document.querySelector(".site-header");
  if (header) {
    window.addEventListener("scroll", function () {
      header.classList.toggle("scrolled", window.scrollY > 10);
    }, { passive: true });
  }

  // ─── Mobile Menu Toggle ───

  var menuBtn = document.querySelector(".menu-toggle");
  var nav = document.querySelector(".site-header nav");
  if (menuBtn && nav) {
    menuBtn.addEventListener("click", function () {
      var isOpen = nav.classList.toggle("open");
      menuBtn.setAttribute("aria-expanded", isOpen);
    });

    // Close menu when a nav link is clicked
    nav.querySelectorAll("a").forEach(function (a) {
      a.addEventListener("click", function () {
        nav.classList.remove("open");
        menuBtn.setAttribute("aria-expanded", "false");
      });
    });
  }

  // ─── Copy-to-Clipboard for Code Blocks ───

  document.querySelectorAll(".copy-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var wrapper = btn.closest(".code-block-wrapper");
      if (!wrapper) return;
      var code = wrapper.querySelector("code");
      if (!code) return;
      var text = code.textContent;

      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(function () {
          showCopied(btn);
        });
      } else {
        // Fallback
        var ta = document.createElement("textarea");
        ta.value = text;
        ta.style.position = "fixed";
        ta.style.opacity = "0";
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand("copy"); showCopied(btn); } catch (e) { /* noop */ }
        document.body.removeChild(ta);
      }
    });
  });

  function showCopied(btn) {
    var original = btn.textContent;
    btn.textContent = "Copied!";
    btn.classList.add("copied");
    setTimeout(function () {
      btn.textContent = original;
      btn.classList.remove("copied");
    }, 1800);
  }

  // ─── Scroll-to-Top Button ───

  var scrollTopBtn = document.querySelector(".scroll-top");
  if (scrollTopBtn) {
    window.addEventListener("scroll", function () {
      scrollTopBtn.classList.toggle("visible", window.scrollY > 400);
    }, { passive: true });

    scrollTopBtn.addEventListener("click", function () {
      window.scrollTo({ top: 0, behavior: "smooth" });
    });
  }
})();
