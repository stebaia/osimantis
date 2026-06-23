import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../domain/entities/person_card.dart';

/// Scheda persona: si compone progressivamente man mano che l'utente racconta.
/// Header (avatar + nome + alias), dati liberi, legami, eventi recenti.
class PersonCardView extends StatelessWidget {
  const PersonCardView({super.key, required this.person});

  final PersonCard person;

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      child: Container(
        decoration: BoxDecoration(
          color: AppColors.surface,
          borderRadius: BorderRadius.circular(24),
          boxShadow: const [
            BoxShadow(color: AppColors.border, blurRadius: 16, offset: Offset(0, 4)),
          ],
        ),
        padding: const EdgeInsets.all(20),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _Header(person: person),
            if (person.data.isNotEmpty) ...[
              const SizedBox(height: 20),
              _DataSection(data: person.data),
            ],
            if (person.neighbors.isNotEmpty) ...[
              const SizedBox(height: 20),
              _Section(
                title: 'Legami',
                child: Column(
                  children: person.neighbors
                      .map((n) => _RelationTile(relation: n))
                      .toList(),
                ),
              ),
            ],
            if (person.events.isNotEmpty) ...[
              const SizedBox(height: 20),
              _Section(
                title: 'Ricordi',
                child: Column(
                  children:
                      person.events.map((e) => _EventTile(event: e)).toList(),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({required this.person});
  final PersonCard person;

  @override
  Widget build(BuildContext context) {
    final initial = person.name.isNotEmpty ? person.name[0].toUpperCase() : '?';
    return Row(
      children: [
        CircleAvatar(
          radius: 28,
          backgroundColor: AppColors.primary.withValues(alpha: 0.15),
          child: Text(
            initial,
            style: const TextStyle(
              fontSize: 24,
              fontWeight: FontWeight.w600,
              color: AppColors.primary,
            ),
          ),
        ),
        const SizedBox(width: 14),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                person.name,
                style: const TextStyle(
                  fontSize: 22,
                  fontWeight: FontWeight.w600,
                  color: AppColors.textPrimary,
                ),
              ),
              if (person.aliases.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 2),
                  child: Text(
                    person.aliases.join(' · '),
                    style: const TextStyle(color: AppColors.textSecondary),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }
}

class _DataSection extends StatelessWidget {
  const _DataSection({required this.data});
  final Map<String, dynamic> data;

  @override
  Widget build(BuildContext context) {
    // is_user è un flag interno, non un dato da mostrare nella scheda.
    final entries = data.entries
        .where((e) => e.key != 'is_user' && e.value != null && '${e.value}'.isNotEmpty)
        .toList();
    if (entries.isEmpty) return const SizedBox.shrink();
    return _Section(
      title: 'Dati',
      child: Column(
        children: entries
            .map(
              (e) => Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(
                      width: 110,
                      child: Text(
                        _label(e.key),
                        style: const TextStyle(color: AppColors.textSecondary),
                      ),
                    ),
                    Expanded(
                      child: Text(
                        '${e.value}',
                        style: const TextStyle(color: AppColors.textPrimary),
                      ),
                    ),
                  ],
                ),
              ),
            )
            .toList(),
      ),
    );
  }

  /// "data_nascita" → "Data nascita".
  String _label(String key) {
    final s = key.replaceAll('_', ' ');
    return s.isEmpty ? s : '${s[0].toUpperCase()}${s.substring(1)}';
  }
}

class _RelationTile extends StatelessWidget {
  const _RelationTile({required this.relation});
  final PersonRelation relation;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          Icon(
            relation.nodeType == 'place' ? Icons.place_outlined : Icons.person_outline,
            size: 18,
            color: AppColors.textSecondary,
          ),
          const SizedBox(width: 8),
          Expanded(
            child: RichText(
              text: TextSpan(
                style: const TextStyle(color: AppColors.textPrimary),
                children: [
                  TextSpan(text: relation.name),
                  if (relation.relation.isNotEmpty)
                    TextSpan(
                      text: '  ·  ${relation.relation}',
                      style: const TextStyle(color: AppColors.textSecondary),
                    ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _EventTile extends StatelessWidget {
  const _EventTile({required this.event});
  final PersonEvent event;

  @override
  Widget build(BuildContext context) {
    final text = event.summary?.isNotEmpty == true ? event.summary! : event.rawText;
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.only(top: 6, right: 8),
            child: CircleAvatar(radius: 3, backgroundColor: AppColors.primary),
          ),
          Expanded(
            child: Text(text, style: const TextStyle(color: AppColors.textPrimary)),
          ),
        ],
      ),
    );
  }
}

class _Section extends StatelessWidget {
  const _Section({required this.title, required this.child});
  final String title;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          title.toUpperCase(),
          style: const TextStyle(
            fontSize: 12,
            letterSpacing: 0.6,
            fontWeight: FontWeight.w600,
            color: AppColors.textSecondary,
          ),
        ),
        const SizedBox(height: 10),
        child,
      ],
    );
  }
}
