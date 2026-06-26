import 'package:flutter/material.dart';

import '../../../../core/theme/app_colors.dart';
import '../../domain/entities/graph_data.dart';

/// Il "nodo" del grafo come pillola tappabile: icona del tipo + nome. Le persone
/// sono in accento viola, i luoghi in superficie neutra; il nodo utente è
/// evidenziato. [highlighted] serve a marcare i nodi toccati dall'ultimo
/// messaggio.
class NodeChip extends StatelessWidget {
  const NodeChip({
    super.key,
    required this.node,
    required this.onTap,
    this.highlighted = false,
  });

  final GraphNode node;
  final VoidCallback onTap;
  final bool highlighted;

  @override
  Widget build(BuildContext context) {
    final isPlace = node.isPlace;
    final bg = isPlace ? AppColors.surface : AppColors.primary;
    final fg = isPlace ? AppColors.textPrimary : Colors.white;
    final icon = isPlace
        ? Icons.place
        : node.isUser
            ? Icons.star
            : Icons.person;

    return Material(
      color: bg,
      shape: StadiumBorder(
        side: highlighted
            ? const BorderSide(color: AppColors.primaryDark, width: 2.5)
            : BorderSide(color: AppColors.border),
      ),
      elevation: highlighted ? 4 : 1,
      child: InkWell(
        customBorder: const StadiumBorder(),
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, size: 16, color: fg),
              const SizedBox(width: 6),
              ConstrainedBox(
                constraints: const BoxConstraints(maxWidth: 120),
                child: Text(
                  node.name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(color: fg, fontWeight: FontWeight.w600),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
