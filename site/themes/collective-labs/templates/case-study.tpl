{% extends "layout.tpl" %}

{% block content %}
    {% set site = data.site %}
    {% set case = data.case %}

    <div class="flex-grow bg-raisin-800 z-50">
        <div class="px-4 pt-14 sm:px-20 pb-1">
            <div class="-ml-4 h-16 mb-10 sm:mb-24 flex flex-row items-center justify-between">
                <a href="/" class="logo inline-flex justify-center items-center">
                    <img src="{{ site.logo_path|default:'/img/logo/colabs_logotype_light_v6.svg' }}" class="h-12" alt="logo">
                </a>
            </div>

            <main class="grow flex flex-col justify-center align-middle">
                <div class="mx-auto">
                    <h2 class="uppercase text-xs font-semibold mb-10">
                        <span class="border-b-4 border-raisin-300 pb-2 font-mono">
                            Case Study
                        </span>
                    </h2>
                    <h1 class="font-montserrat font-black text-3xl sm:text-6xl lg:text-8xl text-balance leading-[0.9] text-raisin-200 mb-8 capitalize">
                        {{ case.title }}
                    </h1>
                    {% if case.summary %}
                        <p class="max-w-3xl lg:max-w-4xl text-2xl lg:text-4xl text-pretty">
                            {{ case.summary }}
                        </p>
                    {% endif %}
                </div>
                {% if case.download_href %}
                    <div class="mt-14">
                        <a href="{{ case.download_href }}" download class="px-10 py-5 font-bold bg-raisin-950 hover:bg-raisin-700 border border-black hover:border-raisin-50 transition-all">
                            {{ case.download_label|default:"Download" }}
                        </a>
                    </div>
                {% endif %}
            </main>
        </div>

        <article class="mt-40 pr-0 pt-8 sm:p-8 bg-raisin-50 text-raisin-800">
            <div class="mb-10">
                <div class="grid grid-flow-col grid-rows-2 sm:grid-rows-1 p-4 gap-x-5">
                    {% if case.engagement %}
                        <div class="row-span-1 mx-auto">
                            <div class="uppercase text-xs font-semibold mb-2 text-raisin-500 font-mono">
                                Engagement
                            </div>
                            <h4 class="flex-1 mt-4 mb-6 text-lg lg:text-2xl font-semibold">
                                {{ case.engagement }}
                            </h4>
                        </div>
                    {% endif %}
                    {% if case.industry %}
                        <div class="row-span-1 mx-auto">
                            <div class="uppercase text-xs font-semibold mb-2 text-raisin-500 font-mono">
                                Industry
                            </div>
                            <h4 class="flex-1 mt-4 mb-6 text-lg lg:text-2xl font-semibold">
                                {{ case.industry }}
                            </h4>
                        </div>
                    {% endif %}
                    {% if case.timeline %}
                        <div class="row-span-1 mx-auto">
                            <div class="uppercase text-xs font-semibold mb-2 text-raisin-500 font-mono">
                                Timeline
                            </div>
                            <h4 class="flex-1 mt-4 mb-6 text-lg lg:text-2xl font-semibold">
                                {{ case.timeline }}
                            </h4>
                        </div>
                    {% endif %}
                    {% if case.services %}
                        <div class="row-span-1 mx-auto">
                            <div class="uppercase text-xs font-semibold mb-2 text-raisin-500 font-mono">
                                Services
                            </div>
                            <h4 class="flex-1 mt-4 mb-6 text-lg lg:text-2xl font-semibold">
                                <ul>
                                    {% for service in case.services %}
                                        <li>
                                            {{ service }}
                                        </li>
                                    {% endfor %}
                                </ul>
                            </h4>
                        </div>
                    {% endif %}
                </div>
                <hr class="border-raisin-300">
            </div>

            {% set sections = case.sections %}
            {% if sections %}
                {% for section in sections %}
                    {% set layout = section.layout|default:"simple" %}
                    {% if layout == "intro" %}
                        <div class="p-4">
                            <div class="grid grid-flow-row sm:grid-flow-col gap-8 justify-items-start">
                                <h4 class="uppercase text-xs font-semibold mb-10 mt-7 lg:mt-8 font-mono">
                                    <span class="text-raisin-400 ">{{ section.label|safe }}</span>
                                    {{ section.title|safe }}
                                </h4>
                                <div class="flex flex-col gap-y-6 sm:pr-10 lg:pr-20 lg:mx-56">
                                    {% for subsection in section.subsections %}
                                        <div>
                                            {% if subsection.title %}
                                                {% set heading_classes = "font-montserrat flex-1 mt-4 mb-4 text-xl md:text-2xl lg:text-3xl text-raisin-700 font-extrabold" %}
                                                {% if subsection.list_style == "circle-counter" %}
                                                    {% set heading_classes = "font-montserrat flex-1 mt-4 mb-8 text-xl md:text-2xl lg:text-3xl text-raisin-700 font-extrabold" %}
                                                {% endif %}
                                                <h3 class="{{ heading_classes }}">
                                                    {{ subsection.title|safe }}
                                                </h3>
                                            {% endif %}
                                            {% if subsection.paragraphs %}
                                                {% for paragraph in subsection.paragraphs %}
                                                    <p class="sm:text-lg lg:text-2xl">
                                                        {{ paragraph|safe }}
                                                    </p>
                                                {% endfor %}
                                            {% endif %}
                                            {% if subsection.items %}
                                                {% if subsection.list_style == "circle-counter" %}
                                                    <ol class="circle-counter">
                                                        {% for item in subsection.items %}
                                                            <li class="list-item pl-2 sm:text-lg lg:text-2xl">
                                                                {{ item|safe }}
                                                            </li>
                                                        {% endfor %}
                                                    </ol>
                                                {% else %}
                                                    <ul class="list-disc pl-4">
                                                        {% for item in subsection.items %}
                                                            <li class="sm:text-lg lg:text-2xl">
                                                                {{ item|safe }}
                                                            </li>
                                                        {% endfor %}
                                                    </ul>
                                                {% endif %}
                                            {% endif %}
                                        </div>
                                    {% endfor %}
                                </div>
                            </div>
                        </div>
                    {% elif layout == "lead-columns" %}
                        <div class="pt-20 p-5 pb-10 sm:pt-30 flex flex-col">
                            <span class="text-raisin-800 uppercase text-xs font-semibold pb-4 font-mono">
                                <span class="text-raisin-400 ">{{ section.label|safe }}</span>
                                {{ section.title|safe }}
                            </span>
                            {% if section.lead %}
                                <h3 class="font-montserrat font-bold text-2xl sm:text-4xl lg:text-5xl text-balance leading-[0.9] text-center">
                                    {{ section.lead|safe }}
                                </h3>
                            {% endif %}
                            {% if section.columns %}
                                <div class="grid sm:grid-cols-2 mt-10 gap-x-10 lg:gap-x-32">
                                    {% for column in section.columns %}
                                        <div class="lg:px-20 space-y-4">
                                            {% if column.title %}
                                                <h4 class="flex-1 mt-4 mb-4 text-xl md:text-2xl lg:text-3xl text-raisin-700 font-montserrat font-extrabold">
                                                    {{ column.title|safe }}
                                                </h4>
                                            {% endif %}
                                            {% if column.paragraphs %}
                                                {% for paragraph in column.paragraphs %}
                                                    <p class="text-justify">
                                                        {{ paragraph|safe }}
                                                    </p>
                                                {% endfor %}
                                            {% endif %}
                                        </div>
                                    {% endfor %}
                                </div>
                            {% endif %}
                        </div>
                    {% elif layout == "conclusion" %}
                    {% else %}
                        <div class="p-4 text-raisin-900 text-lg sm:text-xl">
                            {% if section.body_html %}
                                {{ section.body_html|safe }}
                            {% elif section.body %}
                                {{ section.body|safe }}
                            {% else %}
                                {{ case.body_html|safe }}
                            {% endif %}
                        </div>
                    {% endif %}
                {% endfor %}
            {% else %}
                <div class="p-4 text-raisin-900 text-lg sm:text-xl">
                    {{ case.body_html|safe }}
                </div>
            {% endif %}
        </article>

        {% if case.hero_image %}
            <div class="relative group overflow-hidden">
                <img src="{{ case.hero_image }}" class="w-full h-full object-cover transition-transform duration-500 group-hover:scale-105" loading="lazy" alt="{{ case.hero_image_alt|default:'dashbaord' }}">
            </div>
        {% endif %}

        {% if case.features %}
            <article class="pr-0 pt-8 sm:p-8 bg-raisin-50 text-raisin-800">
                <div class="pt-6 p-5 pb-44 sm:pt-10 flex flex-col">
                    <span class="text-raisin-800 uppercase text-xs font-semibold pb-4 font-mono">
                        <span class="text-raisin-400 ">3.0 _</span>
                        Key Features
                    </span>
                    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-8 mx-auto font-medium text-center">
                        {% for feature in case.features %}
                            <div class="bg-white p-8 rounded-lg shadow-xs space-y-4 hover:shadow-md transition-all hover:-translate-y-1">
                                {% if feature.icon %}
                                    {% set icon_classes = feature.icon_class|default:"mx-auto" %}
                                    <img src="{{ feature.icon }}" class="{{ icon_classes }}">
                                {% endif %}
                                <p class="sm:text-lg lg:text-2xl">
                                    {{ feature.text }}
                                </p>
                            </div>
                        {% endfor %}
                    </div>
                </div>
            </article>
        {% endif %}

        {% if case.result_metrics or case.sections %}
            {% set has_conclusion = false %}
            {% for section in case.sections %}
                {% if section.layout == "conclusion" %}
                    {% set has_conclusion = true %}
                {% endif %}
            {% endfor %}

            {% if case.result_metrics or has_conclusion %}
                <article class="bg-raisin-900 pt-10 sm:pt-32 pb-44 px-8 sm:px-6 text-[#fffffa]">
                    {% if case.result_metrics %}
                        <div class="grid grid-flow-row sm:grid-flow-col gap-8 lg:gap-x-32 justify-items-start">
                            <span class="uppercase text-xs font-semibold pb-4 mt-2 font-mono">
                                <span class="text-raisin-400 ">4.0 _</span>
                                The Result
                            </span>
                            <div class="grid sm:grid-cols-2 gap-6 sm:pr-10 lg:pr-20 lg:mr-20">
                                {% for metric in case.result_metrics %}
                                    <div class="space-y-3">
                                        <span class="text-6xl font-bold font-mono">
                                            {{ metric.value }}
                                        </span>
                                        <h5 class="text-2xl font-montserrat font-medium capitalize text-raisin-100">
                                            {{ metric.label }}
                                        </h5>
                                        {% if metric.description %}
                                            <p class="sm:text-lg lg:text-2xl text-raisin-300">
                                                {{ metric.description }}
                                            </p>
                                        {% endif %}
                                    </div>
                                {% endfor %}
                            </div>
                        </div>
                    {% endif %}

                    {% for section in case.sections %}
                        {% if section.layout == "conclusion" %}
                            <div class="grid grid-flow-row sm:grid-flow-col gap-8 lg:gap-x-32 justify-items-start pt-28">
                                <h4 class="uppercase text-xs font-semibold mb-10 mt-2 font-mono">
                                    <span class="text-raisin-400 ">{{ section.label|safe }}</span>
                                    {{ section.title|safe }}
                                </h4>
                                <div class="flex flex-col gap-y-6 sm:pr-10 lg:pr-20 lg:mx-56">
                                    {% if section.paragraphs %}
                                        {% for paragraph in section.paragraphs %}
                                            <p class="font-semibold text-2xl">
                                                {{ paragraph|safe }}
                                            </p>
                                        {% endfor %}
                                    {% endif %}
                                </div>
                            </div>
                        {% endif %}
                    {% endfor %}
                </article>
            {% endif %}
        {% endif %}

        {% if case.technologies %}
            <article class="pr-0 pt-8 sm:p-8 sm:pt-20 bg-raisin-50 text-raisin-100">
                <div class="pt-6 p-5 pb-44 sm:pt-10 flex flex-col">
                    <span class="text-raisin-800 uppercase text-xs font-semibold pb-4 font-mono">
                        <span class="text-raisin-400 ">5.0 _ </span>
                        Technologies
                    </span>
                    <div class="grid grid-cols-1 w-full md:grid-cols-2 lg:grid-cols-3 gap-4 sm:gap-8 mx-auto font-medium select-none">
                        {% for stack in case.technologies %}
                            <div class="bg-raisin-800 p-8 rounded-lg shadow-xs space-y-4 hover:shadow-md transition-all hover:-translate-y-1">
                                <h4 class="flex-1 mb-4 text-xl md:text-2xl lg:text-3xl text-raisin-50 font-montserrat font-extrabold">
                                    {{ stack.category }}
                                </h4>
                                {% if stack.items %}
                                    <ul class="list-none pl-4">
                                        {% for item in stack.items %}
                                            <li class="li-arrow sm:text-lg lg:text-2xl">
                                                {{ item }}
                                            </li>
                                        {% endfor %}
                                    </ul>
                                {% endif %}
                            </div>
                        {% endfor %}
                    </div>
                </div>
            </article>
        {% endif %}

{% if case.related_projects and case.related_projects|length > 0 %}
    <article class="bg-raisin-50 flex flex-col p-4 pb-32 border-b-8 border-b-raisin-950">
        <span class="inline-flex text-raisin-800 uppercase text-xs font-semibold mb-6 font-mono">
            <div class="w-[0.8em] h-[0.8em] bg-current mt-[0.3em] mr-2"></div>
            Related Projects
        </span>
                <div class="grid md:grid-cols-2 grid-cols-1 gap-4">
                    {% for project in case.related_projects %}
                        <a href="{{ project.href }}" class="pointer-wrapper relative group overflow-hidden select-none h-[500px] block">
                            {% if project.image %}
                                <img src="{{ project.image }}" class="w-full h-full object-cover transition-transform duration-500 group-hover:scale-125 group-hover:blur-sm bg-raisin-900" loading="lazy" alt="{{ project.alt|default:project.title }}">
                            {% endif %}
                            <div class="absolute inset-0 bg-raisin-950 bg-opacity-40"></div>
                            <span class="absolute left-4 top-4 uppercase text-xs font-semibold mb-10 w-full">
                                <span class="border-b-2 border-raisin-300 group-hover:border-pink-500 pb-2 font-mono">
                                    Case Study
                                </span>
                            </span>
                            <div class="absolute inset-0 flex flex-col items-center justify-center p-8">
                                <h5 class="text-3xl font-montserrat font-medium capitalize text-white text-center text-shadow shadow-black group-hover:-translate-y-4 transition-all">
                                    {{ project.title }}
                                </h5>
                            </div>
                            <div class="hidden group-hover:flex items-center justify-center w-16 h-16 rounded-full bg-pink-500 text-raisin-900 font-semibold text-xl absolute pointer-events-none custom-pointer">
                                {{ project.pointer|default:'→' }}
                            </div>
                        </a>
                    {% endfor %}
                </div>
            </article>
        {% endif %}
    </div>

    {% include "partials/footer.tpl" %}
{% endblock %}
